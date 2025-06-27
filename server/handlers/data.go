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
)

func filterArticles(ctx context.Context, api models.DocumentsAPI, filters *models.ArticleFilters) (models.Articles, models.Pagination, *models.Response) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, "", models.RespErrUnauthorized()
	}

	subscriptionStates := user.FilterSubscriptionStatesByID(filters.Subscriptions...)
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
	sort := filters.GetSort()

	// Find items matching filters.
	items, pagination, err := api.SearchItems(ctx, query, filters.GetCount(), &sort, filters.Pagination)
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

func subscriptionsNeededFromCtx(ctx context.Context) map[*models.SubscriptionRequest]*models.Feed {
	data, ok := ctx.Value(subscriptionsCtxKey).(map[*models.SubscriptionRequest]*models.Feed)
	if !ok || data == nil {
		return make(map[*models.SubscriptionRequest]*models.Feed)
	}
	return data
}
