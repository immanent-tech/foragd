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
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic/aggregations"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/web/views"
)

func getSubscriptions(ctx context.Context, api FeedsAPI, ids ...models.SubscriptionID) (models.Subscriptions, *models.Response) {
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, models.RespErrUnauthorized()
	}

	var states models.SubscriptionStates[models.SubscriptionID]

	if len(ids) == 0 {
		states = user.GetAllSubscriptionStates()
	} else {
		states = user.FilterSubscriptionStatesByID(ids...)
	}

	// Get customisation details for subscriptions.
	customisations, err := api.GetSubscriptionCustomisations(ctx, slices.Collect(maps.Keys(states))...)
	if err != nil {
		return nil, models.RespErrBackend(err)
	}
	// Get feed data for subscriptions
	feedIDs := make([]models.FeedID, 0, len(states))
	for _, state := range states {
		feedIDs = append(feedIDs, state.GetFeedID())
	}
	feeds, err := api.GetFeeds(ctx, feedIDs...)
	if err != nil {
		return nil, models.RespErrBackend(err)
	}
	// Get unread counts.
	categoryCounts, resp := getSubscriptionUnreadCounts(ctx, api, user.GetAllSubscriptionStatesByFeed())
	if resp != nil {
		return nil, resp
	}
	// Generate subscriptions from data sources.
	subscriptions := make(models.Subscriptions, 0, len(feeds))
	for feed := range slices.Values(feeds) {
		var state *models.SubscriptionState
		for _, s := range states {
			if s.GetFeedID() == feed.GetID() {
				state = s
				break
			}
		}
		subscription, err := models.GenerateSubscription(
			user.GetID(),
			feed,
			customisations.GetCustomisation(state.GetID()),
			state,
			categoryCounts.GetCount(feed.GetID()))
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Could not generate subscription from data.",
				slog.Any("error", err),
			)
			continue
		}
		subscriptions = append(subscriptions, subscription)
	}

	return subscriptions, nil
}

func filterSubscriptions(ctx context.Context, api FeedsAPI, filters *models.Filters) (models.Subscriptions, models.Pagination, *models.Response) {
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, "", models.RespErrUnauthorized()
	}
	states := user.GetAllSubscriptionStatesByFeed()

	if len(states) == 0 {
		return nil, "", &models.Response{StatusCode: 404}
	}

	var feedIDs []models.FeedID
	var subscriptionIDs []models.SubscriptionID

	var unreadCounts *aggregations.TermsAggregationResults
	var resp *models.Response
	// Get unread counts.
	unreadCounts, resp = getSubscriptionUnreadCounts(ctx, api, states)
	if resp != nil {
		return nil, "", resp
	}

	switch filters.View {
	case models.ViewUnread:
		for _, state := range states {
			if !state.IsRead() || unreadCounts.GetCount(state.GetFeedID()) > 0 {
				feedIDs = append(feedIDs, state.GetFeedID())
				subscriptionIDs = append(subscriptionIDs, state.GetID())
			}
		}
	case models.ViewRead:
		for _, state := range states {
			if state.IsRead() && unreadCounts.GetCount(state.GetFeedID()) == 0 {
				feedIDs = append(feedIDs, state.GetFeedID())
				subscriptionIDs = append(subscriptionIDs, state.GetID())
			}
		}
	case models.ViewAll:
		feedIDs = slices.Collect(maps.Keys(user.GetAllSubscriptionStatesByFeed()))
		for _, state := range states {
			subscriptionIDs = append(subscriptionIDs, state.GetID())
		}
	}
	if len(subscriptionIDs) == 0 {
		return nil, "", &models.Response{StatusCode: http.StatusNotFound}
	}

	// Search subscription customisations for matches.
	customisationQuery := query.Bool(
		query.Filter(
			// Must match any of the given feed IDs.
			query.SubscriptionIDs(subscriptionIDs...),
			// Must match one of the given categories (if set).
			query.Categories(filters.Categories...),
		),
	)
	customisations, _, err := api.SearchSubscriptionCustomisations(ctx, customisationQuery, len(subscriptionIDs), nil, nil)
	if err != nil {
		return nil, "", models.RespErrBackend(err)
	}
	customisationIDs := make([]models.SubscriptionID, 0, len(customisations))
	for customisation := range slices.Values(customisations) {
		customisationIDs = append(customisationIDs, customisation.FeedID)
	}
	// Search feeds for matches.
	feedsQuery := query.Bool(
		query.Filter(
			query.Bool(
				query.Should(
					query.FeedIDs(customisationIDs...),
					query.Bool(
						query.Filter(
							query.FeedIDs(feedIDs...),
							query.Categories(filters.Categories...),
						),
					),
				),
			),
		),
	)
	sort := filters.Sort()
	feeds, pagination, err := api.SearchFeeds(ctx, feedsQuery, filters.CountAsInt(), &sort, filters.Pagination)
	if err != nil {
		return nil, "", models.RespErrBackend(err)
	}
	// Generate subscriptions from data sources.
	subscriptions := make(models.Subscriptions, 0, len(feeds))
	for feed := range slices.Values(feeds) {
		var state *models.SubscriptionState
		var count int
		var found bool
		if state, found = states[feed.GetID()]; !found {
			slogctx.FromCtx(ctx).Warn("No subscription state for retrieved feed.",
				slog.String("feed_id", feed.GetID()),
			)
			continue
		}
		if unreadCounts != nil {
			count = unreadCounts.GetCount(feed.GetID())
		}

		subscription, err := models.GenerateSubscription(
			user.GetID(),
			feed,
			customisations.GetCustomisation(state.GetID()),
			state,
			count,
		)
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Could not generate subscription from data.",
				slog.Any("error", err),
			)
			continue
		}
		subscriptions = append(subscriptions, subscription)
	}

	return subscriptions, pagination, nil
}

func getSubscriptionUnreadCounts(ctx context.Context, api FeedsAPI, states models.SubscriptionStates[models.FeedID]) (*aggregations.TermsAggregationResults, *models.Response) {
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, models.RespErrUnauthorized()
	}

	subscriptionQueries := make([]query.Option, 0, len(states))
	for _, state := range states {
		subscriptionQueries = append(subscriptionQueries, subscriptionQueryUnreadItems(user, state))
	}
	query := query.Bool(
		query.Filter(
			query.Bool(
				query.Should(subscriptionQueries...),
			),
		),
	)
	aggResults, resp := api.ItemsAggregation(ctx, query, aggregations.NewTermsAggregation("UnreadCounts", "feed_id", len(states)))
	if resp != nil {
		return nil, resp
	}
	var (
		categoryCounts aggregations.TermsAggregationResults
		err            error
	)
	categoryCounts.StringTermsAggregate, err = aggregations.ExtractAggregation[*types.StringTermsAggregate](aggResults.Aggregations, "UnreadCounts")
	if err != nil {
		return nil, models.NewResponse(http.StatusInternalServerError, fmt.Errorf("could not extract category counts: %w", err))
	}

	return &categoryCounts, nil
}

func filterArticlesBySubscriptions(ctx context.Context, api FeedsAPI, filters *models.Filters, subIDs ...models.SubscriptionID) (models.Articles, models.Pagination, *models.Response) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, "", models.RespErrUnauthorized()
	}

	subscriptionStates := user.FilterSubscriptionStatesByID(subIDs...)
	feedIDs := make([]models.FeedID, 0, len(subscriptionStates))
	for _, state := range subscriptionStates {
		feedIDs = append(feedIDs, state.GetFeedID())
	}

	// Search through items matching any given feeds filters, excluding any read
	// items.
	query := query.Bool(
		query.BoolQueryName("get_items"),
		query.Filter(
			// Must match any of the given feed IDs.
			query.FeedIDs(feedIDs...),
			// Must match any of the given categories.
			query.Categories(filters.Categories...),
			// And should match one feed clause.
			query.Bool(
				query.Should(buildSubscriptionQueries(user, filters.View, slices.Collect(maps.Values(subscriptionStates))...)...),
			),
		),
	)
	sort := filters.Sort()

	// Find items matching filters.
	items, pagination, err := api.SearchItems(ctx, query, filters.CountAsInt(), &sort, filters.Pagination)
	if err != nil {
		return nil, "", models.RespErrBackend(err)
	}
	// Retrieve subscription customisations for feed subscriptions.
	states := user.FilterSubscriptionStatesByFeed(items.GetFeedIDs()...)
	customisations, err := api.GetSubscriptionCustomisations(ctx, models.GetIDsFromStates(states)...)
	if err != nil {
		return nil, "", models.RespErrBackend(err)
	}
	// Create articles from the items.
	articles := make(models.Articles, 0, len(items))
	for item := range slices.Values(items) {
		state := states[item.GetFeedID()]
		customisation := customisations.GetCustomisation(state.GetID())
		article, err := models.GenerateArticle(item, state.GetItemState(item.GetID()), state.GetID(), customisation)
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Could not generate article from data.",
				slog.Any("error", err),
			)
			continue
		}
		articles = append(articles, article)

	}

	return articles, pagination, nil
}

func getArticles(ctx context.Context, api FeedsAPI, itemIDs ...models.ItemID) (models.Articles, *models.Response) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, models.RespErrUnauthorized()
	}

	// Search through items matching any given feeds filters, excluding any read
	// items.
	query := query.Bool(
		query.Filter(
			// Must match any of the given item IDs,
			query.ItemIDs(itemIDs...),
		),
	)

	items, _, err := api.SearchItems(ctx, query, len(itemIDs), nil, nil)
	if err != nil {
		return nil, models.RespErrBackend(err)
	}

	// Retrieve subscription customisations for feed subscriptions.
	states := user.FilterSubscriptionStatesByFeed(items.GetFeedIDs()...)
	customisations, err := api.GetSubscriptionCustomisations(ctx, models.GetIDsFromStates(states)...)
	if err != nil {
		return nil, models.RespErrBackend(err)
	}
	// Create articles from the items.
	articles := make(models.Articles, 0, len(items))
	for item := range slices.Values(items) {
		state := states[item.GetFeedID()]
		customisation := customisations.GetCustomisation(state.GetID())
		article, err := models.GenerateArticle(item, state.GetItemState(item.GetID()), state.GetID(), customisation)
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Could not generate article from data.",
				slog.Any("error", err),
			)
			continue
		}
		articles = append(articles, article)

	}

	return articles, nil
}

// markArticles will update the subscription state in the user object, explicitly marking the given articles for the
// subscription with the given mark.
func markArticles(ctx context.Context, api BackendAPI, mark models.Mark, itemIDs ...models.ItemID) *models.Response {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return models.RespErrUnauthorized()
	}
	// Retrieve full item details.
	items, resp := getItems(ctx, api, itemIDs...)
	if resp != nil {
		return resp
	}
	states := user.GetAllSubscriptionStatesByFeed()
	// Mark each item in the user data.
	switch mark {
	case models.MarkRead:
		for item := range slices.Values(items) {
			states[item.GetFeedID()].MarkItemsRead(item.GetID())
		}
	case models.MarkUnread:
		for item := range slices.Values(items) {
			states[item.GetFeedID()].MarkItemsRead(item.GetID())
		}
	}
	// Update the states in the user object.
	return updateUserSubscriptionStates(ctx, api, slices.Collect(maps.Values(states))...)
}

func getItems(ctx context.Context, api FeedsAPI, itemIDs ...models.ItemID) (models.Items, *models.Response) {
	if len(itemIDs) == 0 {
		slogctx.FromCtx(ctx).Warn("No item IDs given.")
		return nil, &models.Response{StatusCode: http.StatusNoContent}
	}
	// Match the given item IDs.
	query := query.Bool(
		query.Filter(
			query.ItemIDs(itemIDs...),
		),
	)
	items, _, err := api.SearchItems(ctx, query, len(itemIDs), nil, nil)
	if err != nil {
		return nil, models.RespErrBackend(err)
	}

	return items, nil
}

func getItemTopCategories(ctx context.Context, api FeedsAPI, feeds ...models.FeedID) ([]models.Category, *models.Response) {
	query := query.Bool(
		query.Filter(
			// Must match any of the given feed IDs.
			query.FeedIDs(feeds...),
		),
	)
	aggsResult, resp := api.ItemsAggregation(ctx, query, aggregations.NewTermsAggregation("TopCategories", "categories.raw", 10))
	if resp != nil {
		return nil, resp
	}
	var (
		topCategories aggregations.TermsAggregationResults
		err           error
	)
	topCategories.StringTermsAggregate, err = aggregations.ExtractAggregation[*types.StringTermsAggregate](aggsResult.Aggregations, "TopCategories")
	if err != nil {
		return nil, models.NewResponse(http.StatusInternalServerError, fmt.Errorf("could not extract category counts: %w", err))
	}

	return topCategories.BucketNames(), nil
}

func removeSubscriptions(ctx context.Context, api UserAPI, subscriptions ...models.SubscriptionID) *models.Response {
	return nil
	// if len(subscriptions) == 0 {
	// 	return nil
	// }
	// user, found := models.UserFromCtx(ctx)
	// if !found {
	// 	return models.RespInvalidUser()
	// }
	// // Add the subscriptions to the user.
	// user.RemoveSubscriptions(subscriptions...)
	// // Update the user object.
	// return api.UpdateUser(ctx, user.GetID(), map[string]any{
	// 	"subscriptions": user.Subscriptions,
	// 	"updated_at":    time.Now().UTC(),
	// })
}

func markSubscriptions(ctx context.Context, api UserAPI, mark models.Mark, subIDs ...models.SubscriptionID) *models.Response {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return models.RespErrUnauthorized()
	}
	// Set marked at to current timestamp.
	markedAt := time.Now().UTC()
	// Get all user subscription states.
	states := user.GetAllSubscriptionStates()
	// Loop through given subscription IDs and update states.
	for id := range slices.Values(subIDs) {
		if state, found := states[id]; !found {
			slogctx.FromCtx(ctx).Warn("Trying to mark non-existent user subscription.",
				slog.String("subscription_id", id),
			)
			continue
		} else {
			state.Mark(mark, markedAt)
		}
	}
	// Update the user object.
	return updateUserSubscriptionStates(ctx, api, slices.Collect(maps.Values(states))...)
}

func updateUserSubscriptionStates(ctx context.Context, api UserAPI, states ...*models.SubscriptionState) *models.Response {
	if resp := api.UpdateUser(ctx, map[string]any{
		"subscriptions": states,
		"updated_at":    time.Now().UTC(),
	}); resp != nil {
		return resp
	}
	return nil
}

// getHomePageData retrieves the data required to construct the home page content.
//
//nolint:funlen
func getHomePageData(ctx context.Context, api FeedsAPI) (*views.HomePageData, *models.Response) {
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, models.RespErrUnauthorized()
	}

	data := &views.HomePageData{
		Links: make(map[string]models.PageState),
	}

	// Generate links.
	data.Links["subscriptions"] = RestorePageState(ctx, "/home/subscriptions")
	data.Links["articles"] = RestorePageState(ctx, "/home/articles")

	// Get subscriptions.
	subscriptions, _, resp := filterSubscriptions(ctx, api, models.NewFilters())
	if resp != nil && resp.IsNotFound() {
		return data, resp
	}
	if resp != nil {
		return nil, resp
	}
	// Query definition for fetching unread items for all subscriptions.
	query := query.Bool(
		query.BoolQueryName("item_filters"),
		query.Filter(
			// Must match any of the given feed IDs.
			query.FeedIDs(models.GetFeedIDs(slices.Values(subscriptions))...),
			// And should match one feed clause.
			query.Bool(
				query.Should(buildSubscriptionQueries(user, models.ViewUnread)...),
			),
		),
	)

	var aggs []aggregations.Aggregation
	TermsField := "categories.raw"

	// Aggregation definition for fetching the top 10 item categories across all subscriptions.
	SampleField := "feed_id"
	DefaultMaxDocsPerValue := 10
	ShardSize := 1000
	aggs = append(aggs,
		aggregations.Aggregation{
			Name: "top_categories_sample",
			Definition: types.Aggregations{
				DiversifiedSampler: &types.DiversifiedSamplerAggregation{
					Field:           &SampleField,
					MaxDocsPerValue: &DefaultMaxDocsPerValue,
					ShardSize:       &ShardSize,
				},
				Aggregations: map[string]types.Aggregations{
					"top_categories": {
						Terms: &types.TermsAggregation{
							Field: &TermsField,
						},
					},
				},
			},
		},
	)
	// Aggregation definition for fetching the rare item categories across all subscriptions.
	MaxDocCount := int64(5)
	aggs = append(aggs,
		aggregations.Aggregation{
			Name: "rare_categories",
			Definition: types.Aggregations{
				RareTerms: &types.RareTermsAggregation{
					Field:       &TermsField,
					MaxDocCount: &MaxDocCount,
				},
			},
		},
	)

	// Aggregation definition for fetching a set of random items from all subscriptions.
	ItemIDField := "item_id"
	aggs = append(aggs,
		aggregations.Aggregation{
			Name: "random_items",
			Definition: types.Aggregations{
				RandomSampler: &types.RandomSamplerAggregation{
					Probability: 0.1,
				},
				Aggregations: map[string]types.Aggregations{
					"items": {
						Terms: &types.TermsAggregation{
							Field: &ItemIDField,
						},
					},
				},
			},
		},
	)

	// Perform the request.
	aggsResult, err := api.ItemsAggregation(ctx, query, aggs...)
	if err != nil {
		return nil, models.RespErrBackend(err)
	}
	// Add the aggregations to the data
	data.Aggregations = aggsResult.Aggregations

	return data, nil
}

// getHomePageArticles retrieves a list of articles to display on the home page along with other content.
func getHomePageArticles(ctx context.Context, api FeedsAPI, data *views.HomePageData) (models.Articles, *models.Response) {
	// Get the rare categories aggregation.
	randomItemsAgg, err := aggregations.ExtractAggregation[map[string]any](data.Aggregations, "random_items")
	if err != nil {
		return nil, models.RespErrBackend(err)
	}
	// itemsAgg, err := aggregations.ExtractAggregation[*types.StringTermsAggregate](randomItemsAgg, "sterms#items")
	itemsAgg, ok := randomItemsAgg["sterms#items"].(map[string]any)
	if !ok {
		return nil, models.RespErrBackend(err)
	}
	// if err != nil {
	// 	return nil, fmt.Errorf("could not get random items: %w", err)
	// }
	buckets, ok := itemsAgg["buckets"].([]any)
	if !ok {
		return nil, models.RespErrBackend(err)
	}

	itemIDs := make([]models.ItemID, 0, len(buckets))
	for bucket := range slices.Values(buckets) {
		value, ok := bucket.(map[string]any)
		if !ok {
			continue
		}
		key, ok := value["key"].(string)
		if !ok {
			continue
		}
		itemIDs = append(itemIDs, key)
	}

	articles, resp := getArticles(ctx, api, itemIDs...)
	if resp != nil {
		return nil, models.RespErrBackend(err)
	}

	return articles, nil
}

func getSearchSuggestions(ctx context.Context, api FeedsAPI, searchTerms string) (models.Subscriptions, models.Articles, *models.Response) {
	// Retrieve user object.
	// user, found := models.UserFromCtx(ctx)
	// if !found {
	// 	return nil, nil, models.RespInvalidUser()
	// }

	// subscriptionSearch := &query.MSearchOptions{
	// 	Query: query.Build(
	// 		query.Bool(
	// 			query.Filter(
	// 				query.Term("user_id", user.GetID()),
	// 			),
	// 			query.Must(
	// 				query.Match("title", searchTerms),
	// 				query.Match("description", searchTerms),
	// 				query.Match("categories", searchTerms),
	// 			),
	// 		),
	// 	),
	// 	Sort: []types.SortCombinationsVariant{elastic.SortByScore(), elastic.NewFieldSort("published", models.SortOrderDesc)},
	// }

	// itemsSearch := &query.MSearchOptions{
	// 	Query: query.Build(
	// 		query.Bool(
	// 			// query.Filter(
	// 			// 	query.FeedIDs(feedIDs...),
	// 			// ),
	// 			query.Must(
	// 				query.Match("title", searchTerms),
	// 				query.Match("description", searchTerms),
	// 				query.Match("categories", searchTerms),
	// 			),
	// 		),
	// 	),
	// 	Sort: []types.SortCombinationsVariant{elastic.SortByScore(), elastic.NewFieldSort("published", models.SortOrderDesc)},
	// }

	// subscriptions, articles, err := api.MultiSearch(ctx, subscriptionSearch, itemsSearch)
	// if err != nil {
	// 	return nil, nil, models.RespTemporaryIssue("Could not fetch articles. Please try again.", err)
	// }

	return nil, nil, nil
}

func subscriptionRequestsFromCtx(ctx context.Context) models.SubscriptionRequests {
	data := ctx.Value(subscriptionRequestsCtxKey)
	if data == nil {
		return nil
	}
	var requests models.SubscriptionRequests
	switch value := data.(type) {
	case *models.SubscriptionRequest:
		requests = append(requests, value)
	case []*models.SubscriptionRequest:
		requests = append(requests, value...)
	default:
		return nil
	}

	return requests
}

func subscriptionResultsFromCtx(ctx context.Context) map[*models.SubscriptionRequest]*models.UserMessage {
	data, ok := ctx.Value(subscriptionResultsCtxKey).(map[*models.SubscriptionRequest]*models.UserMessage)
	if !ok || data == nil {
		return make(map[*models.SubscriptionRequest]*models.UserMessage)
	}
	return data
}

func subscriptionsFromCtx(ctx context.Context) map[*models.SubscriptionRequest]*models.Feed {
	data, ok := ctx.Value(subscriptionsCtxKey).(map[*models.SubscriptionRequest]*models.Feed)
	if !ok || data == nil {
		return make(map[*models.SubscriptionRequest]*models.Feed)
	}
	return data
}

// buildSubscriptionQueries generates a slices of queries for the given subscriptions, based on the given filters.
func buildSubscriptionQueries(user *models.User, view models.View, states ...*models.SubscriptionState) []query.Option {
	queries := make([]query.Option, 0, len(user.Subscriptions))
	// Work out what query to use based on the state filter.
	if len(states) == 0 {
		states = slices.Collect(maps.Values(user.GetAllSubscriptionStates()))
	}
	switch view {
	case models.ViewRead:
		for _, state := range states {
			queries = append(queries, subscriptionQueryReadItems(user, state))
		}
	case models.ViewAll:
		for _, state := range states {
			queries = append(queries, subscriptionQueryAllItems(user, state))
		}
	case models.ViewUnread:
		fallthrough
	default:
		for _, state := range states {
			queries = append(queries, subscriptionQueryUnreadItems(user, state))
		}
	}
	return queries
}

// subscriptionQueryUnreadItems generates a query for finding unread items for the given subscription.
func subscriptionQueryUnreadItems(user *models.User, subscription *models.SubscriptionState) query.Option {
	var since time.Time
	if subscription.IsRead() {
		// Match the item if it is published/updated since last time subscription was marked read.
		since = subscription.GetMarkedRead()
	} else {
		// Match the item if it is published/updated since the max user history window.
		since = user.GetMaxHistory()
	}

	return query.Bool(
		query.BoolQueryName(subscription.GetFeedID()+"_unread_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", subscription.GetFeedID()),
			query.Bool(
				query.Should(
					query.Since("published", since),
					query.Since("updated", since),
					query.ItemIDs(subscription.GetUnreadItems()...),
				),
				// Must not match any read items for the feed
				query.MustNot(
					query.ItemIDs(subscription.GetReadItems()...),
				),
			),
		),
	)
}

// subscriptionQueryReadItems generates a query for finding read items for the given subscription.
func subscriptionQueryReadItems(user *models.User, subscription *models.SubscriptionState) query.Option {
	maxHistory := user.GetMaxHistory()

	switch {
	case !subscription.IsRead():
		return query.Bool(
			query.BoolQueryName(subscription.GetFeedID()+"_read_items"),
			query.Filter(
				// Must match this feed.
				query.Term("feed_id", subscription.GetFeedID()),
				// And be published/updated since the user max history.
				query.Bool(
					query.Should(
						// query.Since("published", maxHistory),
						// query.Since("updated", maxHistory),
						query.ItemIDs(subscription.GetReadItems()...),
					),
					// Must not match any unread items for the feed
					query.MustNot(
						query.ItemIDs(subscription.GetUnreadItems()...),
					),
				),
			),
		)
	default:
		return query.Bool(
			query.BoolQueryName(subscription.GetFeedID()+"_read_items"),
			query.Filter(
				// Must match this feed.
				query.Term("feed_id", subscription.GetFeedID()),
				// And should be between the user max history and last read time.
				query.Bool(
					query.Should(
						query.Between("published", maxHistory, subscription.GetMarkedRead()),
						query.Between("updated", maxHistory, subscription.GetMarkedRead()),
						query.ItemIDs(subscription.GetReadItems()...),
					),
					// Must not match any unread items for the feed
					query.MustNot(
						query.ItemIDs(subscription.GetUnreadItems()...),
					),
				),
			),
		)
	}
}

// subscriptionQueryReadItems generates a query for finding all items for the given subscription.
func subscriptionQueryAllItems(user *models.User, subscription *models.SubscriptionState) query.Option {
	maxHistory := user.GetMaxHistory()
	return query.Bool(
		query.BoolQueryName(subscription.GetFeedID()+"_all_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", subscription.GetFeedID()),
			// And be published/updated since the user max history.
			query.Bool(
				query.Should(
					query.Since("published", maxHistory),
					query.Since("updated", maxHistory),
				),
			),
		),
	)
}
