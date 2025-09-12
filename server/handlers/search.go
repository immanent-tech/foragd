// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"

	"github.com/angelofallars/htmx-go"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-feed-me/models"
	"github.com/immanent-tech/go-feed-me/providers/elastic/query"
	"github.com/immanent-tech/go-feed-me/server/forms"
	"github.com/immanent-tech/go-feed-me/web/templates/layouts"
	"github.com/immanent-tech/go-feed-me/web/templates/pages"
	"github.com/immanent-tech/go-feed-me/web/templates/partials"
)

// GetSearchSuggestions performs a search with the user input and presents suggestions back to the user.
func (a *API) GetSearchSuggestions() http.HandlerFunc {
	return alice.New(
		routeLogger,
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
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
		subscriptions, articles, err := a.getSearchSuggestions(req.Context(), request.Text)
		if err != nil {
			slogctx.FromCtx(req.Context()).Debug("Unable to retrieve suggestion data.",
				slog.Any("error", err))
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
		if len(subscriptions) > 0 || len(articles) > 0 {
			// Render suggestions.
			renderPartial(layouts.SearchSuggestions(request, subscriptions, articles), "").ServeHTTP(res, req)
		} else {
			// No suggestions, indicate no change.
			res.WriteHeader(http.StatusNoContent)
		}
	}).ServeHTTP
}

// GetSearchResults performs a search with the user input and renders a page with the search results.
func (a *API) GetSearchResults() http.HandlerFunc {
	return alice.New(
		routeLogger,
	).ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		user, found := models.UserFromCtx(req.Context())
		if !found {
			renderPage(layouts.Drawer(partials.Error(models.NewErrorMessage("No user data", ""))), "").ServeHTTP(res, req)
			return models.ErrUserNotFound
		}
		// Extract the search request.
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			msg := models.NewErrorMessage("Invalid search request",
				"Unable to parse search request. Please check and try again.")
			renderPage(layouts.Drawer(partials.Error(msg)), "").ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		id := request.ID()
		if id == "" {
			msg := models.NewErrorMessage("Invalid search request",
				"Unable to parse search request. Please check and try again.")
			renderPage(layouts.Drawer(partials.Error(msg)), "").ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		// Retrieve favorite data for this search
		fav := user.GetFavorites().FilterByType(models.FavoriteTypeSearch).Get(id)
		// Find subscriptions and articles that match search request.
		subscriptions, articles, err := a.getSearchResults(req.Context(), request.Text)
		switch {
		case err != nil:
			msg := models.NewErrorMessage("Could not generate search results",
				"This could be a temporary problem, please try again.")
			renderPage(layouts.Drawer(partials.Error(msg)), "").ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		case len(subscriptions) > 0 || len(articles) > 0:
			// Render appropriate content.
			template := pages.NewSearchResultsPage(fav, request, subscriptions, articles).Content()
			res.Header().Add(htmx.HeaderReplaceUrl, "/search?"+request.Query())
			renderPage(layouts.Drawer(template), "Search Results - Go Feed Me").ServeHTTP(res, req)
		default:
			template := pages.NoSearchResults()
			res.Header().Add(htmx.HeaderReplaceUrl, "/search?"+request.Query())
			renderPage(layouts.Drawer(template), "Search Results - Go Feed Me").ServeHTTP(res, req)
		}
		return nil
	})).ServeHTTP
}

// getSearchSuggestions will find suggestions for the global search from available subscriptions and articles.
func (a *API) getSearchSuggestions(ctx context.Context, searchTerms string) ([]*partials.Subscription, []*partials.Article, error) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, nil, models.ErrNoUserCtx
	}
	// Get article suggestions.
	feedIDs := user.GetSubscriptionMetadata().GetFeedIDs()
	itemsQuery := query.Bool(
		query.Filter(
			query.Terms("feed_id", feedIDs...),
		),
		query.Must(
			query.Bool(
				query.Should(
					query.SearchAsYouType(searchTerms, "title"),
					query.SearchAsYouType(searchTerms, "description"),
					query.SearchAsYouType(searchTerms, "categories"),
				),
			),
		),
	)
	itemResults, _, err := a.DataAPI().SearchItems(ctx, itemsQuery, 10, &models.SortLastUpdatedDesc, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("findSuggestions: %w", err)
	}
	details, err := models.GenerateArticles(ctx, itemResults)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Error generating articles from items.", slog.Any("error", err))
	}
	articles := make([]*partials.Article, 0, len(itemResults))
	for article := range slices.Values(details) {
		articles = append(articles, partials.NewArticleContent(article))
	}

	// Generate subscriptions from data sources.
	metadataMatches := user.GetSubscriptionMetadata().Search(searchTerms)
	subscriptionMatches, err := a.getSubscriptions(ctx, metadataMatches.GetIDs()...)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Error getting subscriptions.", slog.Any("error", err))
	}
	// Truncate subscription matches to 3 results.
	if len(subscriptionMatches) > 3 {
		subscriptionMatches = subscriptionMatches[:3]
	}
	subscriptions := make([]*partials.Subscription, 0, len(subscriptionMatches))
	for s := range slices.Values(subscriptionMatches) {
		subscriptions = append(subscriptions, partials.NewSubscriptionContent(s))
	}

	return subscriptions, articles, nil
}

// getSearchResults will find suggestions for the global search from available subscriptions and articles.
func (a *API) getSearchResults(ctx context.Context, searchTerms string) ([]*partials.Subscription, []*partials.Article, error) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, nil, models.ErrNoUserCtx
	}
	// Get article suggestions.
	feedIDs := user.GetSubscriptionMetadata().GetFeedIDs()
	itemsQuery := query.Bool(
		query.Filter(
			query.Terms("feed_id", feedIDs...),
		),
		// Must match either: search term in any of the fields, or, matches directly as a search-as-you-type (same as
		// search suggestion).
		query.Must(
			query.Bool(
				query.Should(
					query.MultiMatch(searchTerms, "title", "description", "content", "categories"),
					query.Bool(
						query.Should(
							query.SearchAsYouType(searchTerms, "title"),
							query.SearchAsYouType(searchTerms, "description"),
							query.SearchAsYouType(searchTerms, "categories"),
						),
					),
				),
			),
		),
	)
	itemResults, _, err := a.DataAPI().SearchItems(ctx, itemsQuery, 10, &models.SortLastUpdatedDesc, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("findSuggestions: %w", err)
	}
	details, err := models.GenerateArticles(ctx, itemResults)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Error generating articles from items.", slog.Any("error", err))
	}
	articles := make([]*partials.Article, 0, len(itemResults))
	for article := range slices.Values(details) {
		articles = append(articles, partials.NewArticleContent(article))
	}

	// Generate subscriptions from data sources.
	subscriptions := make([]*partials.Subscription, 0)
	metadataMatches := user.GetSubscriptionMetadata().Search(searchTerms)
	if len(metadataMatches) > 0 {
		subscriptionMatches, err := a.getSubscriptions(ctx, metadataMatches.GetIDs()...)
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Error getting subscriptions.", slog.Any("error", err))
		}
		// Truncate subscription matches to 3 results.
		if len(subscriptionMatches) > 3 {
			subscriptionMatches = subscriptionMatches[:3]
		}
		for s := range slices.Values(subscriptionMatches) {
			subscriptions = append(subscriptions, partials.NewSubscriptionContent(s))
		}
	}
	return subscriptions, articles, nil
}

func (a *API) findSubscriptions(ctx context.Context, request *models.SearchRequest) (models.SubscriptionsSlice, error) {
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, models.ErrNoUserCtx
	}
	// Find subscriptions matching the search request.
	metadataMatches := user.GetSubscriptionMetadata().Search(request.Text)
	subscriptionMatches, err := a.getSubscriptions(ctx, metadataMatches.GetIDs()...)
	if err != nil {
		return nil, fmt.Errorf("findSubscriptions: %w", err)
	}
	return subscriptionMatches, nil
}
