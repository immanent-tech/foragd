// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import "context"

const (
	currentRouteCtxKey  contextKey = "currentRoute"
	previousRouteCtxKey contextKey = "previousRoute"
)

type contextKey string

func CurrentRouteToCtx(ctx context.Context, route Navigation) context.Context {
	return context.WithValue(ctx, currentRouteCtxKey, route)
}

func CurrentRouteFromCtx(ctx context.Context) Navigation {
	route, found := ctx.Value(currentRouteCtxKey).(Navigation)
	if !found {
		return NewNavigation("/home", nil)
	}
	return route
}

func PreviousRouteToCtx(ctx context.Context, route Navigation) context.Context {
	return context.WithValue(ctx, previousRouteCtxKey, route)
}

func PreviousRouteFromCtx(ctx context.Context) Navigation {
	route, found := ctx.Value(previousRouteCtxKey).(Navigation)
	if !found {
		return NewNavigation("/home", nil)
	}
	return route
}
