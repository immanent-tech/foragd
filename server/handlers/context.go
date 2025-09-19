// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"

	"github.com/immanent-tech/foragd/models"
)

const (
	subscriptionFiltersCtxKey contextKey = "subscriptionFilters"
	articleFiltersCtxKey      contextKey = "articleFilters"
)

func subscriptionFiltersToCtx(ctx context.Context, filters models.SubscriptionFilters) context.Context {
	return context.WithValue(ctx, subscriptionFiltersCtxKey, filters)
}

func subscriptionFiltersFromCtx(ctx context.Context) models.SubscriptionFilters {
	if filters, ok := ctx.Value(subscriptionFiltersCtxKey).(models.SubscriptionFilters); ok {
		return filters
	}
	return models.NewSubscriptionFilters()
}

func articleFiltersToCtx(ctx context.Context, filters models.ArticleFilters) context.Context {
	return context.WithValue(ctx, articleFiltersCtxKey, filters)
}

func articleFiltersFromCtx(ctx context.Context) models.ArticleFilters {
	if filters, ok := ctx.Value(articleFiltersCtxKey).(models.ArticleFilters); ok {
		return filters
	}
	return models.NewArticleFilters()
}
