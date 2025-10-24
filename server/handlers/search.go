// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"bufio"
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/justinas/alice"
	"github.com/justinas/nosurf"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/web/templates"
)

// GetSearchSuggestions performs a search with the user input and presents suggestions back to the user.
func (a *API) GetSearchSuggestions() http.HandlerFunc {
	return alice.New().ThenFunc(func(res http.ResponseWriter, req *http.Request) {
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
		user, err := models.UserFromCtx(req.Context())
		ctx := models.CSRFTokenToCtx(req.Context(), nosurf.Token(req))
		if err != nil {
			return fmt.Errorf("unable to display search results: %w", err)
			// renderPage(layouts.Drawer(nil, templates.ErrorPage(models.NewErrorMessage("No user data", ""))), "").ServeHTTP(res, req)
			// return models.ErrUserNotFound
		}
		// Extract the search request.
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			msg := models.NewErrorMessage("Invalid search request",
				"Unable to parse search request. Please check and try again.")
			renderPage(templates.ErrorPage(msg), "").ServeHTTP(res, req.WithContext(ctx))
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		favoriteID := req.FormValue("search_id")
		if favoriteID == "" {
			favoriteID = request.ID()
			if favoriteID == "" {
				msg := models.NewErrorMessage("Invalid search request",
					"Unable to parse search request. Please check and try again.")
				renderPage(templates.ErrorPage(msg), "").ServeHTTP(res, req.WithContext(ctx))
				return models.NewAPIError(err, http.StatusUnprocessableEntity)
			}
		}
		// Retrieve favorite data for this search
		fav := user.GetFavorite(favoriteID)
		// Check if the favorite needs to be updated.
		if fav != nil {
			err := user.UpdateFavoriteSearch(fav.Nickname, request)
			if err != nil {
				template := templates.Notification(
					models.NewErrorMessage("Unable to process favorite.", "Temporary backend issue, please try again."), 0)
				renderPartial(template).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusInternalServerError)
			}
			err = a.DataAPI().UpdateUser(req.Context(), user.GetID(), map[string]any{
				"favorites": user.Favorites,
			})
			if err != nil {
				template := templates.Notification(
					models.NewWarningMessage("Unable to update favorite.", "Temporary backend issue, please try again."), 5*time.Second)
				renderPartial(template).ServeHTTP(res, req)
			}
		}
		// Find subscriptions and articles that match search request.
		subscriptions, articles, err := models.GetSearchResults(req.Context(), a.Elastic, request)
		switch {
		case err != nil:
			msg := models.NewErrorMessage("Could not generate search results",
				"This could be a temporary problem, please try again.")
			renderPage(templates.ErrorPage(msg), "").ServeHTTP(res, req.WithContext(ctx))
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		case len(subscriptions) > 0 || len(articles) > 0:
			slog.Info("templated")
			var template templ.Component
			if IsHTMX(req) {
				template = templ.Join(
					templates.NewSearchResultsPage(user, fav, request, subscriptions, articles).Content(),
					templates.FavoritesList(user.GetAllFavorites(), models.OOBSwapTrue),
				)
			} else {
				template = templates.NewSearchResultsPage(user, fav, request, subscriptions, articles).Content()
			}
			res.Header().Add(htmx.HeaderReplaceUrl, "/search?"+request.Query())
			renderPage(template, templates.GeneratePageTitle("Search Results")).ServeHTTP(res, req.WithContext(ctx))
		default:
			template := templates.NoSearchResults()
			res.Header().Add(htmx.HeaderReplaceUrl, "/search?"+request.Query())
			renderPage(template, templates.GeneratePageTitle("Search Results")).ServeHTTP(res, req.WithContext(ctx))
		}
		return nil
	})).ServeHTTP
}

// WatchSearchResults handles watching the search results for any updates and rendering a notification to the user to refresh the page.
//
//nolint:gocognit
func WatchSearchResults(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Set up SSE connection.
		res.Header().Set("Content-Type", "text/event-stream")
		res.Header().Set("Cache-Control", "no-cache")
		res.Header().Set("Connection", "keep-alive")
		if f, ok := res.(http.Flusher); ok {
			f.Flush()
		} else {
			slogctx.FromCtx(req.Context()).Warn("Cannot flush update stream!")
			res.WriteHeader(http.StatusNoContent)
		}
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
		// Run query once and count results.
		var (
			currentCount int64
			prevCount    int64
		)
		prevCount, err = api.CountItems(req.Context(), query)
		if err != nil {
			return fmt.Errorf("unable to get search request updates: %w", err)
		}
		// Loop while the connection is alive, running the query, counting results, comparing against previous count and
		// notifying user if changed.
		for {
			select {
			case <-req.Context().Done():
				res.Header().Set("Connection", "close")
				res.WriteHeader(http.StatusRequestTimeout)
				return nil
			default:
				currentCount, err = api.CountItems(req.Context(), query)
				if err != nil {
					slogctx.FromCtx(req.Context()).Warn("Cannot get updates count.",
						slog.Any("error", err))
					continue
				}
				// Show updates toast if new items found.
				if currentCount > prevCount {
					slogctx.FromCtx(req.Context()).Debug("Subscription updates found.")
					var b bytes.Buffer //nolint:varnamelen
					template := bufio.NewWriter(&b)
					err := templates.UpdatesToast().Render(req.Context(), template)
					if err != nil {
						slogctx.FromCtx(req.Context()).Warn("Unable to render template.",
							slog.Any("error", err))
						continue
					}
					err = template.Flush()
					if err != nil {
						slogctx.FromCtx(req.Context()).Error("Failed to flush SSE message buffer.",
							slog.Any("error", err))
					}
					_, err = fmt.Fprintf(res, "data: %s\n\n", b.String())
					if err != nil {
						slogctx.FromCtx(req.Context()).Error("Failed to send update SSE message.",
							slog.Any("error", err))
					}
					if f, ok := res.(http.Flusher); ok {
						f.Flush()
					}
				}
				prevCount = currentCount
				time.Sleep(defaultUpdateInterval)
			}
		}
	})).ServeHTTP
}

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
