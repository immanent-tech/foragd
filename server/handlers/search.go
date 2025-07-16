// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"

	"github.com/a-h/templ"
	"github.com/davecgh/go-spew/spew"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/providers/elastic/results"
	"github.com/joshuar/go-feed-me/server/forms"
	"github.com/joshuar/go-feed-me/web/views"
)

// GetSearchSuggestions performs a search with the user input and presents suggestions back to the user.
func (a *API) GetSearchSuggestions() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Set up handler chain.
		chain := alice.New(
			RouteLogger,
		)
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			spew.Dump(err, valid)
			chain.Then(RenderResponse(RespInvalidInput(err))).ServeHTTP(res, req)
			return
		}
		if request.Text == "" {
			res.WriteHeader(http.StatusNoContent)
			return
		}

		subscriptions, articles, err := a.matchObjectsToSearchRequest(req.Context(), request)
		if err != nil {
			chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
			return
		} else if len(subscriptions) > 0 || len(articles) > 0 {
			suggestions := make([]templ.Component, 0, len(articles)+1)

			if len(subscriptions) > 0 {
				// Add subscription suggestions.
				suggestions = append(suggestions, views.SearchSuggestionHeader("Subscriptions"))
				for subscription := range slices.Values(subscriptions) {
					suggestions = append(suggestions, subscription.ShowAsSearchSuggestion())
				}
			}
			if len(articles) > 0 {
				// Add article suggestions.
				suggestions = append(suggestions, views.SearchSuggestionHeader("Articles"))
				for article := range slices.Values(articles) {
					suggestions = append(suggestions, article.ShowAsSearchSuggestion())
				}
			}
			resp := models.NewResponse(
				models.WithResponseTemplate(views.SearchSuggestions(suggestions...)),
			)
			alice.New(
				RouteLogger,
			).Then(RenderResponse(resp)).ServeHTTP(res, req)
		}
	}
}

// GetSearchResults performs a search with the user input and renders a page with the search results.
func (a *API) GetSearchResults() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Set up handler chain.
		chain := alice.New(
			RouteLogger,
		)
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			chain.Then(RenderResponse(RespInvalidInput(err))).ServeHTTP(res, req)
			return
		}
		// Find subscriptions and articles that match search request.
		subscriptions, articles, err := a.matchObjectsToSearchRequest(req.Context(), request)
		if err != nil {
			chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
			return
		} else if len(subscriptions) > 0 || len(articles) > 0 {
			resp := models.NewResponse(
				models.WithResponseTemplate(views.SearchResultsPage(subscriptions, articles)),
			)
			chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
		}
	}
}

//nolint:gocyclo,funlen
func (a *API) matchObjectsToSearchRequest(ctx context.Context, request *models.SearchRequest) ([]*views.Subscription, []*views.Article, error) {
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, nil, models.ErrNoUserCtx
	}

	// Perform search.
	msearchResults, err := a.findSuggestions(ctx, request.Text)
	if err != nil {
		return nil, nil, fmt.Errorf("matchObjectsToSearchRequest: %w", err)
	}
	// Extract the matches.
	var (
		feeds models.Feeds
		items models.Items
	)
	customisations, err := results.GetHits[*models.SubscriptionState]("subscriptions", msearchResults)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Error fetching subscription hits.", slog.Any("error", err))
	}
	feeds, err = results.GetHits[*models.Feed]("feeds", msearchResults)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Error fetching feed hits.", slog.Any("error", err))
	}
	items, err = results.GetHits[*models.Item]("items", msearchResults)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Error fetching item hits.", slog.Any("error", err))
	}
	// Generate subscriptions from data sources.
	subscriptions := make([]*views.Subscription, 0, len(feeds))
	maxSubscriptionResults := 10
	for idx, customisation := range customisations {
		var feed *models.Feed
		if fidx := slices.IndexFunc(feeds, func(f *models.Feed) bool {
			// Feed already fetched in msearch results, use it.
			return f.GetID() == customisation.GetFeedID()
		}); fidx != -1 {
			feed = feeds[fidx]
		} else {
			// Get feed details.
			f, err := a.DataAPI().GetFeeds(ctx, customisation.GetFeedID())
			if err != nil {
				continue
			}
			feed = f[0]
		}
		subscription, err := models.GenerateSubscription(
			customisation,
			feed,
			0,
		)
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Could not generate subscription from data.",
				slog.Any("error", err),
			)
			continue
		}
		if idx == maxSubscriptionResults {
			break
		}
		subscriptions = append(subscriptions, views.NewSubscriptionContent(subscription))
	}
	// Make subscriptions from the feed results up to maxObjectResults - customisationResults.
	states, err := a.getSubscriptionStates(ctx, slices.Collect(maps.Values(user.GetSubscriptionsByFeedID(feeds.GetIDs()...)))...)
	if err != nil {
		return nil, nil, fmt.Errorf("matchObjectsToSearchRequest: %w", err)
	}
	for idx, feed := range feeds {
		var state *models.SubscriptionState
		if state = states.GetByFeedID(feed.GetID()); state == nil {
			slogctx.FromCtx(ctx).Warn("No subscription state for retrieved feed.",
				slog.String("feed_id", feed.GetID()),
			)
			continue
		}
		subscription, err := models.GenerateSubscription(
			state,
			feed,
			0,
		)
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Could not generate subscription from data.",
				slog.Any("error", err),
			)
			continue
		}
		if idx == (maxSubscriptionResults - len(customisations)) {
			break
		}
		subscriptions = append(subscriptions, views.NewSubscriptionContent(subscription))
	}

	articles := make([]*views.Article, 0, len(items))
	states, err = a.getSubscriptionStates(ctx, slices.Collect(maps.Keys(user.GetSubscriptionsByFeedID(items.GetIDs()...)))...)
	if err != nil {
		return nil, nil, fmt.Errorf("matchObjectsToSearchRequest: %w", err)
	}
	for item := range slices.Values(items) {
		var state *models.SubscriptionState
		if state = states.GetByFeedID(item.GetFeedID()); state == nil {
			slogctx.FromCtx(ctx).Warn("No subscription state for retrieved item.",
				slog.String("item_id", item.GetID()),
			)
			continue
		}

		article, err := models.GenerateArticle(item, state)
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Could not generate article from data.",
				slog.Any("error", err),
			)
			continue
		}
		articles = append(articles, views.NewArticleContent(article))
	}

	return subscriptions, articles, nil
}

//nolint:funlen
func (a *API) findSuggestions(ctx context.Context, searchTerms string) (results.MSearchResults, error) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, models.ErrNoUserCtx
	}

	subscriptionsQuery := &query.MsearchSearch{
		Name:  "subscriptions",
		Index: elastic.SubscriptionsIndexFromCtx(ctx),
		Query: query.Build(
			query.Bool(
				query.Filter(
					query.Term("user_id", user.GetID()),
				),
				query.Must(
					query.Bool(
						query.Should(
							query.SearchAsYouType(searchTerms, "customisation.title"),
							query.SearchAsYouType(searchTerms, "customisation.categories"),
						),
					),
				),
			),
		),
	}

	feedIDs := slices.Collect(maps.Keys(user.GetSubscriptionsByFeedID()))
	feedsQuery := &query.MsearchSearch{
		Name:  "feeds",
		Index: elastic.FeedsIndexFromCtx(ctx),
		Query: query.Build(
			query.Bool(
				query.Filter(
					query.Terms("feed_id", feedIDs...),
				),
				query.Must(
					query.Bool(
						query.Should(
							query.SearchAsYouType(searchTerms, "title"),
							query.SearchAsYouType(searchTerms, "description"),
							query.SearchAsYouType(searchTerms, "content"),
							query.SearchAsYouType(searchTerms, "categories"),
						),
					),
				),
			),
		),
	}

	articlesQuery := &query.MsearchSearch{
		Name:  "items",
		Index: elastic.ItemsIndexFromCtx(ctx),
		Query: query.Build(
			query.Bool(
				query.Filter(
					query.Terms("feed_id", feedIDs...),
				),
				query.Must(
					query.Bool(
						query.Should(
							query.SearchAsYouType(searchTerms, "title"),
							query.SearchAsYouType(searchTerms, "description"),
							query.SearchAsYouType(searchTerms, "content"),
							query.SearchAsYouType(searchTerms, "categories"),
						),
					),
				),
			),
		),
	}

	results, err := elastic.MultiSearch(ctx, a.DataAPI().GetAPI(), subscriptionsQuery, feedsQuery, articlesQuery)
	if err != nil {
		return nil, fmt.Errorf("findSuggestions: %w", err)
	}

	return results, nil
}
