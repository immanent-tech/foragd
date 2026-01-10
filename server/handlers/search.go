// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
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

// GetSearchSuggestions performs a search with the user input and presents suggestions back to the user.
func GetSearchSuggestions() http.HandlerFunc {
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
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			slogctx.FromCtx(req.Context()).Debug("Get user data failed.",
				slog.Any("error", err))
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

		renderPartial(
			templates.NewPartial(templates.SearchSuggestions(search, subscriptions, articles)),
		).ServeHTTP(res, req)
	}).ServeHTTP
}

// GetSearchResults performs a search with the user input and renders a page with the search results.
func GetSearchResults() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(showOnError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract the search search.
		search, valid, err := forms.DecodeForm[*models.SearchRequest](req)
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

		// Embed the request in the context.
		ctx := models.SearchRequestToCtx(req.Context(), *search)
		// If the search request has subscription filters, get subscription details.
		if len(search.Subscriptions) > 0 {
			var subscriptions models.Subscriptions
			subscriptions, err = models.GetSubscriptions(req.Context(),
				models.GetSubscriptionsByIDs(search.Subscriptions...),
				models.GetSubscriptionsDynamicInfo(true),
			)
			if err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("get subscriptions: %w", err),
					StatusCode:    http.StatusInternalServerError,
				}
			}
			ctx = models.SubscriptionsToCtx(ctx, subscriptions)
		}

		// Retrieve the user object.
		user, err := models.UserFromCtx(ctx)
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("get user data: %w", err),
				StatusCode:    http.StatusInternalServerError,
			}
		}

		// Retrieve pagination.
		pagination := req.FormValue(models.ParamPagination)
		if err := validation.Validate.Var(pagination, "omitempty,url_encoded"); err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("get pagination: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
			}
		}

		// Find articles that match search request.
		var articles models.Articles
		query, err := models.BuildSearchResultsQuery(ctx, user, search, models.SearchResultsClause(search))
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("build search query: %w", err),
				StatusCode:    http.StatusInternalServerError,
			}
		}
		itemResults, pagination, err := models.SearchItems(
			ctx,
			query,
			defaultArticleResultsCount,
			&search.Sort,
			pagination,
		)
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("search items: %w", err),
				StatusCode:    http.StatusInternalServerError,
			}
		}
		if len(itemResults) > 0 {
			articles, err = models.GenerateArticles(ctx, itemResults)
			if err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("generate articles: %w", err),
					StatusCode:    http.StatusInternalServerError,
				}
			}
		}

		if strings.HasSuffix(chi.RouteContext(ctx).RoutePattern(), "/paginate") {
			if len(articles) > 0 {
				renderPartial(
					templates.NewPartial(templates.SearchResults(articles, pagination)),
				).ServeHTTP(res, req.WithContext(ctx))
			} else {
				res.WriteHeader(http.StatusNoContent)
			}
			return nil
		}
		var template templ.Component
		if len(articles) > 0 {
			// Pagination request, just display next set of results.
			template = templates.SearchResultsGrid(search, articles, pagination)
		} else {
			template = templates.NoSearchResults()
		}
		if htmx.IsHTMX(req) {
			template = templ.Join(template, templates.SearchFilters(templ.Attributes{"hx-swap-oob": "true"}))
		}
		res.Header().Add(htmx.HeaderPushURL, "/search?"+search.Query())
		renderPage(
			templates.NewPage(
				wrapContent(req, template),
				templates.WithPageTitle("Search Results"),
			),
		).ServeHTTP(res, req)

		return nil
	})).ServeHTTP
}

// WatchSearchResults handles watching the search results for any updates and rendering a notification to the user to refresh the page.
func WatchSearchResults() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		// Get user data.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("get user data: %w", err),
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
