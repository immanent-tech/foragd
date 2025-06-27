// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"

	"github.com/joshuar/go-feed-me/models"
)

const (
	subscriptionsCtxKey contextKey = "subscriptions"
	articlesCtxKey      contextKey = "articles"
	paginationCtxKey    contextKey = "pagination"
)

func ArticlesToCtx(ctx context.Context, articles models.Articles) context.Context {
	return context.WithValue(ctx, articlesCtxKey, articles)
}

func ArticlesFromCtx(ctx context.Context) models.Articles {
	if articles, ok := ctx.Value(articlesCtxKey).(models.Articles); ok {
		return articles
	}
	return nil
}

func SubscriptionsToCtx(ctx context.Context, subscriptions models.Subscriptions) context.Context {
	return context.WithValue(ctx, subscriptionsCtxKey, subscriptions)
}

func SubscriptionsFromCtx(ctx context.Context) models.Subscriptions {
	if subscriptions, ok := ctx.Value(subscriptionsCtxKey).(models.Subscriptions); ok {
		return subscriptions
	}
	return nil
}

func PaginationToCtx(ctx context.Context, pagination *models.Pagination) context.Context {
	return context.WithValue(ctx, paginationCtxKey, pagination)
}

func PaginationFromCtx(ctx context.Context) *models.Pagination {
	if pagination, ok := ctx.Value(paginationCtxKey).(*models.Pagination); ok {
		return pagination
	}
	return nil
}
