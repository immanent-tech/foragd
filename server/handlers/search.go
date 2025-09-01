// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-feed-me/models"
	"github.com/immanent-tech/go-feed-me/providers/elastic/query"
	"github.com/immanent-tech/go-feed-me/server/forms"
	"github.com/immanent-tech/go-feed-me/web/templates"
	"github.com/immanent-tech/go-feed-me/web/templates/layouts"
	"github.com/immanent-tech/go-feed-me/web/templates/pages"
	"github.com/immanent-tech/go-feed-me/web/templates/partials"
)

// GetSearchSuggestions performs a search with the user input and presents suggestions back to the user.
func (a *API) GetSearchSuggestions() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Set up handler chain.
		chain := alice.New(
			routeLogger,
		)
		// Extract the search request.
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			chain.Then(render(RespInvalidInput(err))).ServeHTTP(res, req)
			return
		}
		if request.Text == "" {
			res.WriteHeader(http.StatusNoContent)
			return
		}
		// Get results.
		subscriptions, articles, err := a.getSearchSuggestions(req.Context(), request.Text)
		if err != nil {
			chain.Then(render(RespBackendError(err))).ServeHTTP(res, req)
			return
		}
		if len(subscriptions) > 0 || len(articles) > 0 {
			// Render suggestions.
			resp := models.NewResponse(
				models.WithResponseTemplate(layouts.SearchSuggestions(request, subscriptions, articles)),
			)
			alice.New(
				routeLogger,
			).Then(render(resp)).ServeHTTP(res, req)
		} else {
			// No suggestions, indicate no change.
			res.WriteHeader(http.StatusNoContent)
		}
	}
}

// GetSearchResults performs a search with the user input and renders a page with the search results.
func (a *API) GetSearchResults() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Set up handler chain.
		chain := alice.New(
			routeLogger,
		)
		user, found := models.UserFromCtx(req.Context())
		if !found {
			render(RespForbidden()).ServeHTTP(res, req)
			return
		}
		// Extract the search request.
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			chain.Then(render(RespInvalidInput(err))).ServeHTTP(res, req)
			return
		}
		id := request.ID()
		if id == "" {
			render(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		// Retrieve favorite data for this search
		fav := user.GetFavorites().FilterByType(models.FavoriteTypeSearch).Get(id)
		// Find subscriptions and articles that match search request.
		subscriptions, articles, err := a.getSearchResults(req.Context(), request.Text)
		if err != nil {
			chain.Then(render(RespBackendError(err))).ServeHTTP(res, req)
			return
		} else if len(subscriptions) > 0 || len(articles) > 0 {
			var template templ.Component
			// Render appropriate content.
			page := pages.NewSearchResultsPage(fav, request, subscriptions, articles)
			switch {
			case htmx.IsHTMX(req) && !htmx.IsHistoryRestoreRequest(req):
				// Just show content.
				template = page.Content()
			default:
				// Show full page.
				template = templates.Page(
					"Search Results - Go Feed Me",
					layouts.Drawer(page.Content()),
				)
			}
			chain.Then(render(
				models.NewResponse(models.WithResponseTemplate(template)),
			)).ServeHTTP(res, req.WithContext(htmxRespToCtx(req.Context(), htmx.NewResponse().ReplaceURL("/search?"+request.Query()))))
		}
	}
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
		query.Must(
			query.Bool(
				query.Should(
					query.Match("title", searchTerms),
					query.Match("description", searchTerms),
					query.Match("content", searchTerms),
					query.Match("categories", searchTerms),
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
