// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"slices"

	"github.com/a-h/templ"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic/results"
	"github.com/joshuar/go-feed-me/server/forms"
	"github.com/joshuar/go-feed-me/web/views"
)

// GetSearchSuggestions performs a search with the user input and presents suggestions back to the user.
func GetSearchSuggestions(api models.DocumentsAPI) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			RenderError(res, req, models.NewResponse(http.StatusBadRequest, err))
			return
		}

		subscriptions, articles, resp := matchObjectsToSearchRequest(req.Context(), api, request)
		if resp != nil {
			slogctx.FromCtx(req.Context()).Warn("Search suggestions failed.", slog.Any("error", resp.Error()))
			RenderError(res, req, models.RespErrBackend(err))
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

			ctx := templateToCtx(req.Context(), views.SearchSuggestions(suggestions...))
			alice.New(
				RouteLogger,
			).Then(RenderTemplate()).ServeHTTP(res, req.WithContext(ctx))

		}
	}
}

// GetSearchResults performs a search with the user input and renders a page with the search results.
func GetSearchResults(api models.DocumentsAPI) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			RenderError(res, req, models.NewResponse(http.StatusBadRequest, err))
			return
		}

		subscriptions, articles, resp := matchObjectsToSearchRequest(req.Context(), api, request)
		if resp != nil {
			slogctx.FromCtx(req.Context()).Warn("Search suggestions failed.", slog.Any("error", resp.Error()))
			RenderError(res, req, models.RespErrBackend(err))
			return
		} else if len(subscriptions) > 0 || len(articles) > 0 {

			ctx := templateToCtx(req.Context(), views.SearchResultsPage(subscriptions, articles))
			alice.New(
				RouteLogger,
			).Then(RenderTemplate()).ServeHTTP(res, req.WithContext(ctx))
		}
	}
}

func matchObjectsToSearchRequest(ctx context.Context, api models.DocumentsAPI, request *models.SearchRequest) ([]*views.Subscription, []*views.Article, *models.Response) {
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, nil, models.RespErrUnauthorized()
	}

	states := user.GetAllSubscriptionStatesByFeed()

	// Perform search.
	msearchResults, err := api.FindSuggestions(ctx, request.Text)
	if err != nil {
		return nil, nil, models.RespErrBackend(err)
	}
	// Extract the matches.
	customisations, _ := results.GetHits[*models.SubscriptionCustomisation]("customisations", msearchResults)
	feeds, _ := results.GetHits[*models.Feed]("feeds", msearchResults)
	items, _ := results.GetHits[*models.Item]("items", msearchResults)
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
			f, err := api.GetFeeds(ctx, customisation.GetFeedID())
			if err != nil {
				continue
			}
			feed = f[0]
		}
		subscription, err := models.GenerateSubscription(
			user.GetID(),
			feed,
			customisation,
			states[customisation.GetFeedID()],
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
	for idx, feed := range feeds {
		var state *models.SubscriptionState
		var found bool
		if state, found = states[feed.GetID()]; !found {
			slogctx.FromCtx(ctx).Warn("No subscription state for retrieved feed.",
				slog.String("feed_id", feed.GetID()),
			)
			continue
		}
		subscription, err := models.GenerateSubscription(
			user.GetID(),
			feed,
			nil,
			state,
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
	for item := range slices.Values(items) {
		var state *models.SubscriptionState
		var found bool
		if state, found = states[item.GetFeedID()]; !found {
			slogctx.FromCtx(ctx).Warn("No subscription state for retrieved item.",
				slog.String("item_id", item.GetID()),
			)
			continue
		}
		var customisation *models.SubscriptionCustomisation
		if cidx := slices.IndexFunc(customisations, func(c *models.SubscriptionCustomisation) bool {
			// Feed already fetched in msearch results, use it.
			return c.GetFeedID() == item.GetFeedID()
		}); cidx != -1 {
			customisation = customisations[cidx]
		} else {
			// Get customisation details.
			c, err := api.GetSubscriptionCustomisations(ctx, state.GetID())
			if err != nil {
				continue
			}
			if len(c) > 0 {
				customisation = c[0]
			}
		}

		article, err := models.GenerateArticle(item, state.GetItemState(item.GetID()), state.GetID(), customisation)
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
