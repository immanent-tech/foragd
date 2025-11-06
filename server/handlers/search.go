// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/web/templates"
)

// GetSearchSuggestions performs a search with the user input and presents suggestions back to the user.
func (a *API) GetSearchSuggestions() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			slogctx.FromCtx(req.Context()).Debug("Get search suggestions failed.",
				slog.Any("error", err))
			res.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		if request.Text == "" {
			res.WriteHeader(http.StatusNoContent)
			return
		}
		// Get results.
		articles, err := models.GetSearchSuggestions(req.Context(), a.Elastic, request.Text)
		if err != nil {
			slogctx.FromCtx(req.Context()).Debug("Get search suggestions failed.",
				slog.Any("error", err))
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
		if len(articles) > 0 {
			// Render suggestions.
			renderPartial(templates.SearchSuggestions(request, articles)).ServeHTTP(res, req)
		} else {
			// No suggestions, indicate no change.
			res.WriteHeader(http.StatusNoContent)
		}
	}).ServeHTTP
}

// GetSearchResults performs a search with the user input and renders a page with the search results.
func (a *API) GetSearchResults() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		pageTitle := templates.GeneratePageTitle("Search Results")
		// Extract the search request.
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			renderPage(templates.NotFound(), pageTitle).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				http.StatusUnprocessableEntity,
			)
		}
		pagination := req.FormValue(models.ParamPagination)

		ctx := models.SearchRequestToCtx(req.Context(), *request)

		var articles models.Articles
		var template templ.Component
		// Find subscriptions and articles that match search request.
		articles, pagination, err = models.GetSearchResults(ctx, a.Elastic, request, pagination)
		if err != nil {
			msg := models.NewErrorMessage("Unable to process request", "This might be a temporary issue, please try again.")
			renderPage(templates.ErrorPage(msg), pageTitle).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("unable to retrieve subscriptions: %w", err),
				http.StatusInternalServerError,
			)
		}
		if strings.HasSuffix(chi.RouteContext(ctx).RoutePattern(), "/paginate") {
			// Pagination request, just display next set of results.
			switch {
			case len(articles) > 0:
				renderPartial(templates.ResultsList(articles, pagination)).ServeHTTP(res, req.WithContext(ctx))
			default:
				res.WriteHeader(http.StatusNoContent)
				return nil
			}
		} else {
			// Generate appropriate template.
			switch {
			case len(articles) > 0:
				template = templates.SearchResults(request, articles, pagination)
			default:
				template = templates.NoSearchResults()
			}
			if IsHTMX(req) {
				template = templ.Join(template, templates.SearchFilters(templ.Attributes{"hx-swap-oob": "true"}))
			}
			res.Header().Add(htmx.HeaderReplaceUrl, "/search?"+request.Query())
			renderPage(template, templates.GeneratePageTitle("Search Results")).ServeHTTP(res, req.WithContext(ctx))
		}
		return nil
	})).ServeHTTP
}

// WatchSearchResults handles watching the search results for any updates and rendering a notification to the user to refresh the page.
func WatchSearchResults(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Get user data.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return fmt.Errorf("unable to get user data: %w", err)
		}
		// Extract the search request.
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			return fmt.Errorf("unable to get search request updates: %w", err)
		}
		// Build query.
		query := models.BuildSearchResultsQuery(user, request)
		// Watch for updates to search results.
		watchForUpdates(api, query).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// AddSubscriptionFilter handles adding a subscription as a search filter.
func AddSubscriptionFilter() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		data := req.FormValue("subscription-filter-select")
		if data != "" {
			values := strings.Split(data, "|")
			renderPartial(templates.SubscriptionFilter(values[0], values[1])).ServeHTTP(res, req)
		}
		res.WriteHeader(http.StatusOK)
		return nil
	})).ServeHTTP
}
