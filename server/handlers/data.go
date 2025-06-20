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

func filterArticlesBySubscriptions(ctx context.Context, api models.DocumentsAPI, filters *models.Filters, subIDs ...models.SubscriptionID) (models.Articles, models.Pagination, *models.Response) {
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
			query.Terms("feed_id", feedIDs...),
			// Must match any of the given categories.
			query.Terms("categories.raw", filters.Categories...),
			// And should match one feed clause.
			query.Bool(
				query.Should(models.BuildSubscriptionQueries(user, filters.View, slices.Collect(maps.Values(subscriptionStates))...)...),
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

func getArticles(ctx context.Context, api models.DocumentsAPI, itemIDs ...models.ItemID) (models.Articles, *models.Response) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, models.RespErrUnauthorized()
	}

	// Search through items matching any given feeds filters, excluding any read
	// items.
	query := query.Bool(
		query.Filter(
			// Must match any of the given item IDs,
			query.Terms("item_id", itemIDs...),
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
func markArticles(ctx context.Context, api models.DocumentsAPI, mark models.Mark, itemIDs ...models.ItemID) *models.Response {
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
	return api.UpdateUser(ctx, map[string]any{
		"subscriptions": slices.Collect(maps.Values(states)),
	})
}

func getItems(ctx context.Context, api models.DocumentsAPI, itemIDs ...models.ItemID) (models.Items, *models.Response) {
	if len(itemIDs) == 0 {
		slogctx.FromCtx(ctx).Warn("No item IDs given.")
		return nil, &models.Response{StatusCode: http.StatusNoContent}
	}
	// Match the given item IDs.
	query := query.Bool(
		query.Filter(
			query.Terms("item_id", itemIDs...),
		),
	)
	items, _, err := api.SearchItems(ctx, query, len(itemIDs), nil, nil)
	if err != nil {
		return nil, models.RespErrBackend(err)
	}

	return items, nil
}

func getItemTopCategories(ctx context.Context, api models.DocumentsAPI, feeds ...models.FeedID) ([]models.Category, *models.Response) {
	query := query.Bool(
		query.Filter(
			// Must match any of the given feed IDs.
			query.Terms("feed_id", feeds...),
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

func markSubscriptions(ctx context.Context, api models.DocumentsAPI, mark models.Mark, subIDs ...models.SubscriptionID) *models.Response {
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
	return api.UpdateUser(ctx, map[string]any{
		"subscriptions": slices.Collect(maps.Values(states)),
	})
}

// getHomePageData retrieves the data required to construct the home page content.
//
//nolint:funlen
func getHomePageData(ctx context.Context, api models.DocumentsAPI) (*views.HomePageData, *models.Response) {
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
	subscriptions, _, resp := models.FilterSubscriptions(ctx, api, models.NewFilters())
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
			query.Terms("feed_id", models.GetFeedIDs(slices.Values(subscriptions))...),
			// And should match one feed clause.
			query.Bool(
				query.Should(models.BuildSubscriptionQueries(user, models.ViewUnread)...),
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
func getHomePageArticles(ctx context.Context, api models.DocumentsAPI, data *views.HomePageData) (models.Articles, *models.Response) {
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

func getSearchSuggestions(ctx context.Context, api models.DocumentsAPI, searchTerms string) (models.Subscriptions, models.Articles, *models.Response) {
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
