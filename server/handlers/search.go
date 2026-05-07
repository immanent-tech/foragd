// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	estypes "github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/service"
	"github.com/immanent-tech/foragd/validation"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/element"
)

const (
	defaultSubscriptionSuggestionsCount = 3
	defaultArticleSuggestionsCount      = 10
	defaultArticleResultsCount          = 15
)

type SearchSuggestions struct {
	template templ.Component
}

func (h *SearchSuggestions) PartialResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(h.template).ServeHTTP(res, req)
}

// HandleSearchSuggestions performs a search with the user input and presents suggestions back to the user.
func HandleSearchSuggestions() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Decode search.
		search, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			slogctx.FromCtx(req.Context()).Debug("Get search suggestions failed.",
				slog.Any("error", err))
			res.WriteHeader(http.StatusUnprocessableEntity)
			return
		}

		// Ignore empty text string.
		if search.Text == "" {
			res.WriteHeader(http.StatusNoContent)
			return
		}

		// Retrieve the user object.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			slogctx.FromCtx(req.Context()).Debug("Get user data failed.",
				slog.Any("error", models.ErrCtxValueNotFound))
			res.WriteHeader(http.StatusInternalServerError)
			return
		}

		searchJobs, jobCtx := errgroup.WithContext(req.Context())
		defer jobCtx.Done()

		var subscriptions models.Subscriptions
		var articles models.Articles
		sort := models.SortMostRelevant

		// Generate subscription suggestions.
		searchJobs.Go(func() error {
			subscriptions, err = service.GetSubscriptionSuggestions(
				jobCtx,
				search.Text,
				defaultSubscriptionSuggestionsCount,
				nil,
			)
			if err != nil {
				slogctx.FromCtx(jobCtx).Debug("Get search suggestions: unable to get subscription suggestions.",
					slog.Any("error", err))
			}
			return nil
		})

		// Generate article suggestions.
		searchJobs.Go(func() error {
			searchOption, err := service.BuildSearchResultsQuery(
				req.Context(),
				user,
				search,
				models.SearchSuggestionsClause(search),
			)
			if err != nil {
				slogctx.FromCtx(jobCtx).Debug("Unable to build search results query.",
					slog.Any("error", err))
				return nil
			}
			var itemResults models.Items
			itemResults, _, err = models.SearchItems(
				jobCtx,
				searchOption,
				defaultArticleSuggestionsCount,
				&sort,
				nil,
			)
			if err != nil {
				slogctx.FromCtx(jobCtx).Debug("Unable to search articles.",
					slog.Any("error", err))
			}
			if len(itemResults) > 0 {
				articles, err = service.GenerateArticles(jobCtx, itemResults)
				if err != nil {
					slogctx.FromCtx(jobCtx).Debug("Unable to generate articles.",
						slog.Any("error", err))
				}
			}
			return nil
		})

		err = searchJobs.Wait()
		if err != nil {
			slogctx.FromCtx(req.Context()).Warn("Get search suggestions: run background jobs failed.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusInternalServerError)
		}

		// Generate search suggestions object.
		RenderPartial(&SearchSuggestions{
			template: templates.SearchSuggestions(&models.SearchResults{
				Search:        *search,
				Subscriptions: subscriptions,
				Articles:      articles,
			}),
		}).ServeHTTP(res, req)
	}).ServeHTTP
}

type SearchResults struct {
	results *models.SearchResults
}

func (h *SearchResults) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(templates.SearchResults(h.results),
			templates.WithPageTitle("Search Results"),
		)).ServeHTTP(res, req)
}

func (h *SearchResults) PartialResponse(res http.ResponseWriter, req *http.Request) {
	switch req.URL.Path {
	case "/search":
		res.Header().Add(htmx.HeaderPushURL, "/search?"+h.results.Search.Query())
		template := templates.SearchResults(h.results)
		if len(h.results.Articles) > 0 {
			// Also update the search filters element.
			template = templ.Join(
				template,
				templates.AdvancedSearch(&h.results.Search, templ.Attributes{"hx-swap-oob": "true"}),
			)
		}
		templ.Handler(template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
		templ.Handler(templates.UpdateTitle("Search Results")).ServeHTTP(res, req)
		templ.Handler(templates.SideBar(templ.Attributes{"hx-swap-oob": "true"})).ServeHTTP(res, req)
		templ.Handler(templates.Dock(templ.Attributes{"hx-swap-oob": "true"})).ServeHTTP(res, req)
	case "/search/paginate":
		if len(h.results.Articles) == 0 {
			res.WriteHeader(http.StatusNoContent)
		} else {
			templ.Handler(templates.SearchResults(h.results), templ.WithFragments(templates.PaginateFragment)).
				ServeHTTP(res, req)
		}
	}
}

// HandleSearchResults performs a search with the user input and renders a page with the search results.
func HandleSearchResults() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Extract the search search.
		search, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Invalid search request",
					"Please check the search request data and try again",
				),
			}).ServeHTTP(res, req)
			return
		}

		// Retrieve the user object.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound),
				StatusCode:    http.StatusInternalServerError,
			}).ServeHTTP(res, req)
			return
		}

		ctx := req.Context()

		// If the search request has subscription filters, get subscription details.
		if len(search.Subscriptions) > 0 {
			subscriptionFilters := models.NewListDisplayFilters()
			subscriptionFilters.Subscriptions = search.Subscriptions
			subscriptionFilters.View = search.View
			subscriptions, _, err := service.FilterSubscriptions(req.Context(), user, &subscriptionFilters, "")
			if err != nil && !errors.Is(err, models.ErrNotFound) {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("get subscriptions: %w", err),
					StatusCode:    http.StatusInternalServerError,
				}).ServeHTTP(res, req)
				return
			}
			ctx = models.SubscriptionsToCtx(ctx, subscriptions)
		}

		// Retrieve pagination.
		pagination := req.FormValue(models.ParamPagination)
		if err := validation.Validate.Var(pagination, "omitempty,url_encoded"); err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("get pagination: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
			}).ServeHTTP(res, req)
		}

		// Find articles that match search request.
		var articles models.Articles
		var categories []models.Category
		searchQuery, err := service.BuildSearchResultsQuery(ctx, user, search, models.SearchResultsClause(search))
		if err != nil && !errors.Is(err, models.ErrNotFound) {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("build articles search query: %w", err),
				StatusCode:    http.StatusInternalServerError,
			}).ServeHTTP(res, req)
			return
		}

		if searchQuery != nil {
			searchJobs, jobCtx := errgroup.WithContext(req.Context())
			defer jobCtx.Done()

			// Search for articles.
			searchJobs.Go(func() error {
				var items models.Items
				items, pagination, err = models.SearchItems(
					ctx,
					searchQuery,
					defaultArticleResultsCount,
					&search.Sort,
					&pagination,
				)
				if err != nil {
					return fmt.Errorf("search articles: %w", err)
				}
				if len(items) > 0 {
					articles, err = service.GenerateArticles(ctx, items)
					if err != nil {
						return fmt.Errorf("generate articles: %w", err)
					}
				}
				return nil
			})

			// Generate top categories for articles.
			searchJobs.Go(func() error {
				termsField := "categories.raw"
				termsCount := 10
				// Perform aggregation.
				resp, err := elastic.Search[*models.Item](ctx,
					schema.ItemsIndexRO(),
					// Use the original search query but filter out common categories.
					query.Bool(
						query.Must(searchQuery),
						query.MustNot(
							query.Terms(
								"categories.raw",
								slices.Concat(models.CommonCategoryFilters, []string{""}),
							),
						),
					),
					elastic.WithAggregations(
						elastic.Aggs{
							"TopCategories": estypes.Aggregations{
								Terms: &estypes.TermsAggregation{
									Field: &termsField,
									Size:  &termsCount,
								},
							},
						},
					),
					elastic.WithSize(0),
					elastic.WithDocSorting(),
				)
				if err != nil {
					return fmt.Errorf("aggregate articles: %w", err)
				}

				topCategoriesAgg, ok := resp.Aggregations["TopCategories"].(*estypes.StringTermsAggregate)
				if !ok {
					return fmt.Errorf("extract aggregation: %w", models.ErrInvalidAPIResult)
				}
				topCategoriesBuckets, ok := topCategoriesAgg.Buckets.([]estypes.StringTermsBucket)
				if !ok {
					return fmt.Errorf("extract buckets: %w", models.ErrInvalidAPIResult)
				}

				for bucket := range slices.Values(topCategoriesBuckets) {
					if category, ok := bucket.Key.(models.Category); ok {
						categories = append(categories, category)
					}
				}

				return nil
			})

			// Run background requests in parallel and wait for results.
			err = searchJobs.Wait()
			if err != nil {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("search items: %w", err),
					StatusCode:    http.StatusInternalServerError,
				}).ServeHTTP(res, req)
				return
			}
		}

		RenderInternalPage(&SearchResults{
			results: &models.SearchResults{
				Search:     *search,
				Articles:   articles,
				Categories: categories,
				Pagination: &pagination,
			},
		}).ServeHTTP(res, req)
	}).ServeHTTP
}

// HandleSearchUpdates handles checking for any new results for the search request and notifying the user.
func HandleSearchUpdates() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Extract the search request.
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			slogctx.FromCtx(req.Context()).Error("Failed to extract search request.",
				slog.Any("error", models.ErrCtxValueNotFound),
			)
			res.WriteHeader(http.StatusNoContent)
			return
		}

		// Build query.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			slogctx.FromCtx(req.Context()).Error("Failed to get user data.",
				slog.Any("error", models.ErrCtxValueNotFound),
			)
			res.WriteHeader(http.StatusNoContent)
			return
		}
		// Override the published within on the search request to last 5 minutes for updates.
		request.PublishedWithin = models.SearchRequestPublishedWithinLast5mins
		updatesQuery, err := service.BuildSearchResultsQuery(
			req.Context(),
			user,
			request,
			models.SearchResultsClause(request),
		)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Cannot build search query.",
				slog.Any("error", models.ErrCtxValueNotFound),
			)
			res.WriteHeader(http.StatusNoContent)
			return
		}

		// Count items matching.
		updateCount, err := models.CountItems(req.Context(), updatesQuery)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Failed to get updates.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusNoContent)
			return
		}

		// If updates found, render a notification.
		if updateCount > 0 {
			RenderPartial(&PartialTemplate{template: templates.UpdatesToast(
				element.WithHXOptions(
					element.WithHXMethod(http.MethodGet, "/search"),
					element.WithHXTarget(templates.ContentID.Target()),
					element.WithHXSwap("innerHTML scroll:top transition:true"),
					element.WithHXPushURL(true),
					element.WithHXValues(request.Values()),
				),
			)}).ServeHTTP(res, req)
		} else {
			res.WriteHeader(http.StatusNoContent)
		}
	}).ServeHTTP
}

// AddSubscriptionFilter handles adding a subscription as a search filter.
func AddSubscriptionFilter() http.HandlerFunc {
	return alice.New().ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		subscription, valid, err := forms.DecodeForm[*models.AddSubscriptionSearchFilterRequest](req)
		if err != nil || !valid {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Invalid search request",
					"Please check the search request data and try again",
				),
			}).ServeHTTP(res, req)
		}
		RenderPartial(&PartialTemplate{
			template: templates.AddSearchSubscriptionFilter(subscription),
		}).ServeHTTP(res, req)
	}).ServeHTTP
}

// GetSubscriptionFilterSuggestions handles showing a list of subscriptions as suggestions when building a search query.
func GetSubscriptionFilterSuggestions() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		defaultSuggestionCount := 10
		suggestion, valid, err := forms.DecodeForm[*models.GetSubscriptionsSuggestionRequest](req)
		if err != nil || !valid {
			slogctx.FromCtx(req.Context()).Error("Invalid subscription suggestion input.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusNoContent)
			return
		}
		subscriptions, err := service.GetSubscriptionSuggestions(
			req.Context(),
			suggestion.Text,
			defaultSuggestionCount,
			nil,
		)
		if err != nil && !errors.Is(err, models.ErrNotFound) {
			slogctx.FromCtx(req.Context()).Error("Unable to get subscription suggestions.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
		if errors.Is(err, models.ErrNotFound) {
			res.WriteHeader(http.StatusNoContent)
			return
		}
		RenderPartial(&PartialTemplate{
			template: templates.SearchSubscriptionFilterSuggestions(subscriptions),
		}).ServeHTTP(res, req)
	}).ServeHTTP
}
