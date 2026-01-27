// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

const (
	userCtxKey          contextKey = "user"
	filtersCtxKey       contextKey = "filters_"
	pathCtxKey          contextKey = "req_path"
	subscriptionsCtxKey contextKey = "subscriptions"
)

type contextKey string

var ErrCtxValueNotFound = &APIError{
	InternalError: errors.New("context value not found"),
	StatusCode:    http.StatusNotFound,
}

// UserToCtx stores a user in the context.
func UserToCtx(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userCtxKey, user)
}

// UserFromCtx retrieves a user from the context, if any.
func UserFromCtx(ctx context.Context) (*User, error) {
	user, found := ctx.Value(userCtxKey).(*User)
	if !found {
		return nil, fmt.Errorf("get user from context: %w", ErrCtxValueNotFound)
	}
	return user, nil
}

// PageFiltersToCtx stores the current page display filters in the context.
func PageFiltersToCtx(ctx context.Context, path string, filters Filters) context.Context {
	return context.WithValue(ctx, filtersCtxKey+contextKey(path), filters)
}

// PageFiltersFromCtx retrieves the current page display filters from the context.
func PageFiltersFromCtx(ctx context.Context, path string) *ListFilters {
	filters, found := ctx.Value(filtersCtxKey + contextKey(path)).(*ListFilters)
	if !found {
		newFilters := NewListDisplayFilters()
		return &newFilters
	}
	return filters
}

// PathToCtx stores the URL path in the context.
func PathToCtx(ctx context.Context, path string) context.Context {
	return context.WithValue(ctx, pathCtxKey, path)
}

// PathFromCtx retrieves the URL path from the context.
func PathFromCtx(ctx context.Context) string {
	path, found := ctx.Value(pathCtxKey).(string)
	if !found {
		return ""
	}
	return path
}

// SubscriptionsToCtx stores the slice of Subscriptions in the context. Useful for pre-fetching/generating subscriptions
// for later usage.
func SubscriptionsToCtx(ctx context.Context, subscriptions Subscriptions) context.Context {
	return context.WithValue(ctx, subscriptionsCtxKey, subscriptions)
}

// SubscriptionsFromCtx retrieves the slice of Subscriptions from the context. If no Subscriptions slice is in the
// context, it returns an empty slice.
func SubscriptionsFromCtx(ctx context.Context) Subscriptions {
	subscriptions, found := ctx.Value(subscriptionsCtxKey).(Subscriptions)
	if !found {
		return make(Subscriptions, 0)
	}
	return subscriptions
}
