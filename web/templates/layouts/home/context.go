// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import "context"

const (
	currentRouteCtxKey  contextKey = "currentRoute"
	previousRouteCtxKey contextKey = "previousRoute"
)

type contextKey string

func CurrentRouteToCtx(ctx context.Context, route Route) context.Context {
	return context.WithValue(ctx, currentRouteCtxKey, route)
}

func CurrentRouteFromCtx(ctx context.Context) Route {
	route, found := ctx.Value(currentRouteCtxKey).(Route)
	if !found {
		return Route{Path: "/home"}
	}
	return route
}

func PreviousRouteToCtx(ctx context.Context, route Route) context.Context {
	return context.WithValue(ctx, previousRouteCtxKey, route)
}

func PreviousRouteFromCtx(ctx context.Context) Route {
	route, found := ctx.Value(previousRouteCtxKey).(Route)
	if !found {
		return Route{Path: "/home"}
	}
	return route
}
