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
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/providers/elastic/results"
	"github.com/joshuar/go-feed-me/server/forms"
	"github.com/joshuar/go-feed-me/web/templates/partials"
	"github.com/joshuar/go-feed-me/web/templates/views"
)

// GetSearchSuggestions performs a search with the user input and presents suggestions back to the user.
func (a *API) GetSearchSuggestions() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Set up handler chain.
		chain := alice.New(
			RouteLogger,
		)
		// Extract the search request.
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
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
		}
		if len(subscriptions) > 0 || len(articles) > 0 {
			// Show the suggestions.
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
			RouteLogger,
		)
		user, found := models.UserFromCtx(req.Context())
		if !found {
			RenderResponse(RespForbidden()).ServeHTTP(res, req)
			return
		}
		// Extract the search request.
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			chain.Then(RenderResponse(RespInvalidInput(err))).ServeHTTP(res, req)
			return
		}
		id := request.ID()
		if id == "" {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		// Retrieve favorite data for this search
		fav := user.GetFavorites().FilterByType(models.FavoriteTypeSearch).Get(id)
		// Find subscriptions and articles that match search request.
		subscriptions, articles, err := a.matchObjectsToSearchRequest(req.Context(), request)
		if err != nil {
			chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
			return
		} else if len(subscriptions) > 0 || len(articles) > 0 {
			resp := models.NewResponse(
				models.WithResponseTemplate(views.NewSearchResultsPage(fav, request, subscriptions, articles).Template(req)),
			)
			chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
		}
	}
}

//nolint:funlen
func (a *API) matchObjectsToSearchRequest(ctx context.Context, request *models.SearchRequest) ([]*partials.Subscription, []*views.Article, error) {
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
		items models.Items
	)
	items, err = results.GetHits[*models.Item]("items", msearchResults)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Error fetching item hits.", slog.Any("error", err))
	}
	articles := make([]*views.Article, 0, len(items))
	allMetadata := user.GetSubscriptionMetadata().FilterByFeedIDs(items.GetFeedIDs()...)
	for item := range slices.Values(items) {
		var state *models.SubscriptionMetadata
		if state = allMetadata.GetByFeedID(item.GetFeedID()); state == nil {
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

	// Generate subscriptions from data sources.
	metadataMatches := user.GetSubscriptionMetadata().Search(request.Text)
	subscriptionMatches, err := a.getSubscriptions(ctx, metadataMatches.GetIDs()...)
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

func (a *API) findSuggestions(ctx context.Context, searchTerms string) (results.MSearchResults, error) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, models.ErrNoUserCtx
	}

	// subscriptionsQuery := &query.MsearchSearch{
	// 	Name:  "subscriptions",
	// 	Index: elastic.SubscriptionsIndexFromCtx(ctx),
	// 	Query: query.Build(
	// 		query.Bool(
	// 			query.Filter(
	// 				query.Term("user_id", user.GetID()),
	// 			),
	// 			query.Must(
	// 				query.Bool(
	// 					query.Should(
	// 						query.SearchAsYouType(searchTerms, "customisation.title"),
	// 						query.SearchAsYouType(searchTerms, "customisation.categories"),
	// 					),
	// 				),
	// 			),
	// 		),
	// 	),
	// }

	// feedIDs := slices.Collect(maps.Keys(user.GetSubscriptionsByFeedID()))
	// feedsQuery := &query.MsearchSearch{
	// 	Name:  "feeds",
	// 	Index: elastic.FeedsIndexFromCtx(ctx),
	// 	Query: query.Build(
	// 		query.Bool(
	// 			query.Filter(
	// 				query.Terms("feed_id", feedIDs...),
	// 			),
	// 			query.Must(
	// 				query.Bool(
	// 					query.Should(
	// 						query.SearchAsYouType(searchTerms, "title"),
	// 						query.SearchAsYouType(searchTerms, "description"),
	// 						query.SearchAsYouType(searchTerms, "content"),
	// 						query.SearchAsYouType(searchTerms, "categories"),
	// 					),
	// 				),
	// 			),
	// 		),
	// 	),
	// }

	feedIDs := user.GetSubscriptionMetadata().GetFeedIDs()
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

	results, err := elastic.MultiSearch(ctx, a.DataAPI().GetAPI(), articlesQuery)
	if err != nil {
		return nil, fmt.Errorf("findSuggestions: %w", err)
	}

	return results, nil
}
