// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/angelofallars/htmx-go"
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
			slogctx.FromCtx(req.Context()).Debug("Invalid search suggestion input.",
				slog.Any("error", err))
			res.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		if request.Text == "" {
			res.WriteHeader(http.StatusNoContent)
			return
		}
		// Get results.
		subscriptions, articles, err := models.GetSearchSuggestions(req.Context(), a.Elastic, request.Text)
		if err != nil {
			slogctx.FromCtx(req.Context()).Debug("Unable to retrieve suggestion data.",
				slog.Any("error", err))
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
		if len(subscriptions) > 0 || len(articles) > 0 {
			// Render suggestions.
			renderPartial(templates.SearchSuggestions(request, subscriptions, articles)).ServeHTTP(res, req)
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
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			msg := models.NewErrorMessage("Server could not complete request!", "This might be temporary, please try again.")
			renderPage(templates.ErrorPage(msg), templates.GeneratePageTitle(pageTitle)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to retrieve user data: %w", err), http.StatusInternalServerError)
		}
		// Extract the search request.
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			renderPage(templates.NotFound(), pageTitle).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				http.StatusUnprocessableEntity,
			)
		}
		var favoriteID string
		favoriteID = req.FormValue("search_id")
		if favoriteID == "" {
			favoriteID, err = request.ID()
			if err != nil {
				msg := models.NewErrorMessage("Unable to parse search request", "This might be a temporary issue, please try again.")
				renderPage(templates.ErrorPage(msg), pageTitle).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusUnprocessableEntity)
			}
		}
		// // Retrieve favorite data for this search
		// fav := user.GetFavorite(favoriteID)
		// if fav != nil {
		// 	// Update favorite in user.
		// 	err := user.UpdateFavoriteSearch(fav.Nickname, request)
		// 	if err != nil {
		// 		msg := models.NewErrorMessage("Unable to process request", "This might be a temporary issue, please try again.")
		// 		renderPage(templates.ErrorPage(msg), pageTitle).ServeHTTP(res, req)
		// 		return models.NewAPIError(
		// 			fmt.Errorf("unable to update search favorite: %w", err),
		// 			http.StatusInternalServerError,
		// 		)
		// 	}
		// 	// Update user.
		// 	err = a.DataAPI().UpdateUser(req.Context(), user.GetID(), map[string]any{
		// 		"favorites": user.Favorites,
		// 	})
		// 	if err != nil {
		// 		msg := models.NewErrorMessage("Unable to process request", "This might be a temporary issue, please try again.")
		// 		renderPage(templates.ErrorPage(msg), pageTitle).ServeHTTP(res, req)
		// 		return models.NewAPIError(
		// 			fmt.Errorf("unable to update user data: %w", err),
		// 			http.StatusInternalServerError,
		// 		)
		// 	}
		// }
		// Find subscriptions and articles that match search request.
		subscriptions, articles, err := models.GetSearchResults(req.Context(), a.Elastic, request)
		switch {
		case err != nil:
			msg := models.NewErrorMessage("Unable to process request", "This might be a temporary issue, please try again.")
			renderPage(templates.ErrorPage(msg), pageTitle).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("unable to retrieve subscriptions: %w", err),
				http.StatusInternalServerError,
			)
		case len(subscriptions) > 0 || len(articles) > 0:
			template := templates.NewSearchResultsPage(user, request, subscriptions, articles).Content()
			res.Header().Add(htmx.HeaderReplaceUrl, "/search?"+request.Query())
			renderPage(template, templates.GeneratePageTitle("Search Results")).ServeHTTP(res, req)
		default:
			template := templates.NoSearchResults()
			res.Header().Add(htmx.HeaderReplaceUrl, "/search?"+request.Query())
			renderPage(template, templates.GeneratePageTitle("Search Results")).ServeHTTP(res, req)
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
