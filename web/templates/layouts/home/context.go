// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"context"

	"github.com/joshuar/go-feed-me/models"
)

const (
	currentRouteCtxKey  contextKey = "currentRoute"
	previousRouteCtxKey contextKey = "previousRoute"
	layoutCtxKey        contextKey = "layout"
)

type contextKey string

func CurrentViewToCtx(ctx context.Context, view models.PageView) context.Context {
	return context.WithValue(ctx, currentRouteCtxKey, view)
}

func CurrentViewFromCtx(ctx context.Context) models.PageView {
	route, found := ctx.Value(currentRouteCtxKey).(models.PageView)
	if !found {
		return models.NewPageView("/home", nil)
	}
	return route
}

func PreviousViewToCtx(ctx context.Context, view models.PageView) context.Context {
	return context.WithValue(ctx, previousRouteCtxKey, view)
}

func PreviousViewFromCtx(ctx context.Context) models.PageView {
	route, found := ctx.Value(previousRouteCtxKey).(models.PageView)
	if !found {
		return models.NewPageView("/home", nil)
	}
	return route
}

// FiltersToCtx stores the current filters in the context.
func LayoutToCtx(ctx context.Context, content *HomeLayout) context.Context {
	return context.WithValue(ctx, layoutCtxKey, content)
}

// FiltersFromCtx retrieves the current filters from the context. If there are no filters stored in the context, the
// default filters will be returned.
func LayoutFromCtx(ctx context.Context) *HomeLayout {
	content, found := ctx.Value(layoutCtxKey).(*HomeLayout)
	if !found {
		return nil
	}
	return content
}
