// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/validation"
	"github.com/immanent-tech/foragd/web/templates"
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
	return defaultHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
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
			subscriptions, err = models.GetSubscriptionSuggestions(
				jobCtx,
				search.Text,
				defaultSubscriptionSuggestionsCount,
				models.GetSubscriptionsDynamicInfo(true),
			)
			if err != nil {
				slogctx.FromCtx(jobCtx).Debug("Get search suggestions: unable to get subscription suggestions.",
					slog.Any("error", err))
			}
			return nil
		})

		// Generate article suggestions.
		searchJobs.Go(func() error {
			searchOption, err := models.BuildSearchResultsQuery(
				req.Context(),
				user,
				search,
				models.SearchSuggestionsClause(search),
			)
			if err != nil {
				slogctx.FromCtx(jobCtx).Debug("Get search suggestions: unable to get all subscriptions.",
					slog.Any("error", err))
			}
			var itemResults models.Items
			itemResults, _, err = models.SearchItems(
				jobCtx,
				searchOption,
				defaultArticleSuggestionsCount,
				&sort,
				"",
			)
			if err != nil {
				slogctx.FromCtx(jobCtx).Debug("Get search suggestions: unable to get article suggestions.",
					slog.Any("error", err))
			}
			if len(itemResults) > 0 {
				articles, err = models.GenerateArticles(jobCtx, itemResults)
				if err != nil {
					slogctx.FromCtx(jobCtx).Debug("Get search suggestions: unable to get article suggestions.",
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
	res.Header().Add(htmx.HeaderPushURL, "/search?"+h.results.Search.Query())
	switch req.URL.Path {
	case "/search":
		template := templates.SearchResults(h.results)
		if len(h.results.Articles) > 0 {
			// Also update the search filters element.
			template = templ.Join(
				template,
				templates.SearchFilters(&h.results.Search, templ.Attributes{"hx-swap-oob": "true"}),
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
			templ.Handler(templates.SearchResults(h.results), templ.WithFragments(templates.PaginateSearchFragment)).ServeHTTP(res, req)
		}
	}
}

// HandleSearchResults performs a search with the user input and renders a page with the search results.
func HandleSearchResults() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
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

		ctx := req.Context()

		// If the search request has subscription filters, get subscription details.
		if len(search.Subscriptions) > 0 {
			var subscriptions models.Subscriptions
			subscriptions, err = models.GetSubscriptions(req.Context(),
				models.GetSubscriptionsByIDs(search.Subscriptions...),
				models.GetSubscriptionsDynamicInfo(true),
			)
			if err != nil {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("get subscriptions: %w", err),
					StatusCode:    http.StatusInternalServerError,
				}).ServeHTTP(res, req)
				return
			}
			ctx = models.SubscriptionsToCtx(ctx, subscriptions)
		}

		// Retrieve the user object.
		user := models.UserFromCtx(ctx)
		if user == nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound),
				StatusCode:    http.StatusInternalServerError,
			}).ServeHTTP(res, req)
			return
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
		query, err := models.BuildSearchResultsQuery(ctx, user, search, models.SearchResultsClause(search))
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("build search query: %w", err),
				StatusCode:    http.StatusInternalServerError,
			}).ServeHTTP(res, req)
			return
		}
		itemResults, pagination, err := models.SearchItems(
			ctx,
			query,
			defaultArticleResultsCount,
			&search.Sort,
			pagination,
		)
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("search items: %w", err),
				StatusCode:    http.StatusInternalServerError,
			}).ServeHTTP(res, req)
			return
		}
		if len(itemResults) > 0 {
			articles, err = models.GenerateArticles(ctx, itemResults)
			if err != nil {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("generate articles: %w", err),
					StatusCode:    http.StatusInternalServerError,
				}).ServeHTTP(res, req)
				return
			}
		}

		RenderInternalPage(&SearchResults{
			results: &models.SearchResults{
				Search:     *search,
				Articles:   articles,
				Pagination: pagination,
			},
		}).ServeHTTP(res, req)
	}).ServeHTTP
}

// WatchSearchResults handles watching the search results for any updates and rendering a notification to the user to refresh the page.
func WatchSearchResults() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		// Get user data.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			return &models.APIError{
				InternalError: fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound),
				StatusCode:    http.StatusInternalServerError,
				UserMessage:   models.NewErrorMessage("Unable to watch for search results updates", ""),
			}
		}
		// Extract the search request.
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			return &models.APIError{
				InternalError: fmt.Errorf("decode search request: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage:   models.NewErrorMessage("Unable to watch for search results updates", ""),
			}
		}
		// Build query.
		query, err := models.BuildSearchResultsQuery(req.Context(), user, request, models.SearchResultsClause(request))
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("build search query: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage:   models.NewErrorMessage("Unable to watch for search results updates", ""),
			}
		}
		// Watch for updates to search results.
		watchForUpdates(query).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// AddSubscriptionFilter handles adding a subscription as a search filter.
func AddSubscriptionFilter() http.HandlerFunc {
	return alice.New().ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		subscription, valid, err := forms.DecodeForm[*models.AddSubscriptionSearchFilterRequest](req)
		if err != nil || !valid {
			return &models.APIError{
				InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Invalid search request",
					"Please check the search request data and try again",
				),
			}
		}
		renderPartial(templates.NewPartial(templates.AddSearchSubscriptionFilter(subscription))).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// GetSubscriptionFilterSuggestions handles showing a list of subscriptions as suggestions when building a search query.
func GetSubscriptionFilterSuggestions() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		defaultSuggestionCount := 10
		suggestion, valid, err := forms.DecodeForm[*models.GetSubscriptionsSuggestionRequest](req)
		if err != nil || !valid {
			slogctx.FromCtx(req.Context()).Error("Invalid subscription suggestion input.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusNoContent)
			return
		}
		subscriptions, err := models.GetSubscriptionSuggestions(req.Context(), suggestion.Text, defaultSuggestionCount)
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
		renderPartial(
			templates.NewPartial(templates.SearchSubscriptionFilterSuggestions(subscriptions)),
		).ServeHTTP(res, req)
	}).ServeHTTP
}
