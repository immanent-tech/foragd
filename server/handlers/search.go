// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
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
	"github.com/immanent-tech/foragd/providers/elastic/query"
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
			// renderPage(layouts.Drawer(nil, templates.Error(models.NewErrorMessage("No user data", ""))), "").ServeHTTP(res, req)
			// return models.ErrUserNotFound
		}
		// Extract the search request.
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			msg := models.NewErrorMessage("Invalid search request",
				"Unable to parse search request. Please check and try again.")
			renderPage(templates.Error(msg), "").ServeHTTP(res, req.WithContext(ctx))
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		favoriteID := req.FormValue("search_id")
		if favoriteID == "" {
			favoriteID = request.ID()
			if favoriteID == "" {
				msg := models.NewErrorMessage("Invalid search request",
					"Unable to parse search request. Please check and try again.")
				renderPage(templates.Error(msg), "").ServeHTTP(res, req.WithContext(ctx))
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
		subscriptions, articles, err := a.getSearchResults(req.Context(), request)
		switch {
		case err != nil:
			msg := models.NewErrorMessage("Could not generate search results",
				"This could be a temporary problem, please try again.")
			renderPage(templates.Error(msg), "").ServeHTTP(res, req.WithContext(ctx))
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

// getSearchResults will find suggestions for the global search from available subscriptions and articles.
func (a *API) getSearchResults(ctx context.Context, request *models.SearchRequest) (models.Subscriptions, []*models.Article, error) {
	user, err := models.UserFromCtx(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get search results: %w", err)
	}
	itemResults, _, err := a.DataAPI().SearchItems(ctx, buildSearchQuery(user, request), 10, &models.SortLastUpdatedDesc, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get search results: %w", err)
	}
	articles, err := models.GenerateArticles(ctx, itemResults)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get search results: %w", err)
	}
	// articles := make([]*partials.Article, 0, len(itemResults))
	// for article := range slices.Values(details) {
	// 	articles = append(articles, partials.NewArticleContent(article))
	// }

	// Generate subscriptions from data sources.
	subscriptions := make(models.Subscriptions, 0)
	metadataMatches := user.GetSubscriptionMetadata().Search(request.Text)
	if len(metadataMatches) > 0 {
		subscriptions, err := models.GetSubscriptions(ctx, a.Elastic, metadataMatches.GetIDs()...)
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Error getting subscriptions.", slog.Any("error", err))
		}
		// Truncate subscription matches to 3 results.
		if len(subscriptions) > 3 {
			subscriptions = subscriptions[:3]
		}
		// for s := range slices.Values(subscriptionMatches) {
		// 	subscriptions = append(subscriptions, partials.NewSubscriptionContent(s))
		// }
	}
	return subscriptions, articles, nil
}

func buildSearchQuery(user *models.User, request *models.SearchRequest) query.Option {
	// var err error
	var loc *time.Location
	if request.Timezone != "" {
		loc, _ = time.LoadLocation(request.Timezone)
		// if err != nil {
		// 	slogctx.FromCtx(ctx).Debug("Error parsing timezone in request.",
		// 		slog.Any("error", err))
		// }
	} else {
		loc, _ = time.LoadLocation("UTC")
	}
	var since time.Time
	switch request.PublishedWithin {
	case models.SearchRequestPublishedWithinLastHour:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-time.Hour).Format(time.Layout), loc)
	case models.SearchRequestPublishedWithinLast12hours:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-12*time.Hour).Format(time.Layout), loc)
	case models.SearchRequestPublishedWithinLastDay:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-24*time.Hour).Format(time.Layout), loc)
	case models.SearchRequestPublishedWithinLastWeek:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-7*24*time.Hour).Format(time.Layout), loc)
	case models.SearchRequestPublishedWithinLastMonth:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-30*24*time.Hour).Format(time.Layout), loc)
	}

	subscriptions := user.GetSubscriptionMetadata()
	if len(request.Subscriptions) > 0 {
		subscriptions = subscriptions.FilterByIDs(request.Subscriptions...)
	}

	return query.Bool(
		query.Filter(
			query.Bool(
				query.Should(models.BuildSubscriptionQueries(user, request.View, subscriptions...)...),
			),
			query.Bool(
				query.Should(
					query.Since("published", since),
					query.Since("updated", since),
				),
			),
		),
		// Must match either: search term in any of the fields, or, matches directly as a search-as-you-type (same as
		// search suggestion).
		query.Must(
			// Search across title, description and content fields, with preference for match in that order (via field
			// boosting).
			query.SimpleQueryString(request.Text, "", "title^6", "description^3", "content"),
			// query.Bool(
			// 	query.Should(
			// 		query.MultiMatch(request.Text, "title", "description", "content"),
			// 		query.SimpleQueryString(request.Text, "", "title^6", "description^3", "content"),
			// 	),
			// ),
			query.SimpleQueryString(request.Categories, "", "categories"),
			query.SimpleQueryString(request.Authors, "", "authors", "contributors"),
		),
	)
}

func (a *API) findSubscriptions(ctx context.Context, request *models.SearchRequest) (models.Subscriptions, error) {
	// Retrieve user object.
	user, err := models.UserFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to find subscriptions: %w", err)
	}
	// Find subscriptions matching the search request.
	metadataMatches := user.GetSubscriptionMetadata().Search(request.Text)
	subscriptionMatches, err := models.GetSubscriptions(ctx, a.Elastic, metadataMatches.GetIDs()...)
	if err != nil {
		return nil, fmt.Errorf("unable to find subscriptions: %w", err)
	}
	return subscriptionMatches, nil
}
