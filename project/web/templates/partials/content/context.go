// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package content

import (
	"context"
	"errors"
)

type contextKey string

const (
	backLinkCtxKey     = "backlink"
	outGoingLinkCtxKey = "outgoingLink"
)

var ErrNotInCtx = errors.New("not found in context")

// func FeedsToContext(ctx context.Context, feeds []string) context.Context {
// 	newCtx := context.WithValue(ctx, feedsCtxKey, feeds)

// 	return newCtx
// }

// func FeedsFromContext(ctx context.Context) []string {
// 	feeds, ok := ctx.Value(feedsCtxKey).([]string)
// 	if !ok {
// 		return nil
// 	}

// 	return feeds
// }

// func CategoriesToContext(ctx context.Context, categories []string) context.Context {
// 	newCtx := context.WithValue(ctx, categoriesCtxKey, categories)

// 	return newCtx
// }

// func CategoriesFromCtx(ctx context.Context) []string {
// 	categories, ok := ctx.Value(categoriesCtxKey).([]string)
// 	if !ok {
// 		return nil
// 	}

// 	return categories
// }

func OutGoingLinkToCtx(ctx context.Context, link string) context.Context {
	newCtx := context.WithValue(ctx, outGoingLinkCtxKey, link)

	return newCtx
}

func OutGoingLinkFromCtx(ctx context.Context) string {
	link, ok := ctx.Value(outGoingLinkCtxKey).(string)
	if !ok {
		return ""
	}

	return link
}

func BackLinkToCtx(ctx context.Context, link string) context.Context {
	newCtx := context.WithValue(ctx, backLinkCtxKey, link)

	return newCtx
}

func BackLinkFromCtx(ctx context.Context) string {
	link, ok := ctx.Value(backLinkCtxKey).(string)
	if !ok {
		return ""
	}

	return link
}
