// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic/aggregations"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
)

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
