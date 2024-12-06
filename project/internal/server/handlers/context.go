// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"errors"
)

type contextKey string

const (
	feedsCtxKey      contextKey = "feeds"
	itemsCtxKey      contextKey = "items"
	categoriesCtxKey contextKey = "categories"
)

var ErrNotInCtx = errors.New("config not found in context")

func FeedsToCtx(ctx context.Context, feeds []string) context.Context {
	return arrayToContext(ctx, feedsCtxKey, feeds)
}

func FeedsFromCtx(ctx context.Context) []string {
	return arrayFromContext(ctx, feedsCtxKey)
}

func ItemsToCtx(ctx context.Context, items []string) context.Context {
	return arrayToContext(ctx, itemsCtxKey, items)
}

func ItemsFromCtx(ctx context.Context) []string {
	return arrayFromContext(ctx, itemsCtxKey)
}

func CategoriesToCtx(ctx context.Context, categories []string) context.Context {
	return arrayToContext(ctx, categoriesCtxKey, categories)
}

func CategoriesFromCtx(ctx context.Context) []string {
	return arrayFromContext(ctx, categoriesCtxKey)
}

func arrayToContext(ctx context.Context, key contextKey, value []string) context.Context {
	newCtx := context.WithValue(ctx, key, value)

	return newCtx
}

func arrayFromContext(ctx context.Context, key contextKey) []string {
	value, ok := ctx.Value(key).([]string)
	if !ok {
		return nil
	}

	return value
}
