// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"

	"github.com/joshuar/go-feed-me/models"
)

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
