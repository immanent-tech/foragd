// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
)

const (
	userCtxKey          contextKey = "user"
	filtersCtxKey       contextKey = "filters_"
	csrfTokenCtxKey     contextKey = "csrfToken"
	pathCtxKey          contextKey = "req_path"
	searchRequestCtxKey contextKey = "search_request"
	subscriptionsCtxKey contextKey = "subscriptions"
	dataAPICtxKey       contextKey = "data_api"
)

type contextKey string

// UserToCtx stores a user in the context.
func UserToCtx(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userCtxKey, user)
}

// UserFromCtx retrieves a user from the context, if any.
func UserFromCtx(ctx context.Context) *User {
	user, found := ctx.Value(userCtxKey).(*User)
	if !found {
		return nil
	}
	return user
}

// CSRFTokenToCtx stores the current valid CSRF token in the context.
func CSRFTokenToCtx(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, csrfTokenCtxKey, token)
}

// CSRFTokenFromCtx retrieves the current valid CSRF token from the context.
func CSRFTokenFromCtx(ctx context.Context) string {
	if token, ok := ctx.Value(csrfTokenCtxKey).(string); ok {
		return token
	}
	return ""
}

// PageFiltersToCtx stores the current page display filters in the context.
func PageFiltersToCtx(ctx context.Context, path string, filters Filters) context.Context {
	return context.WithValue(ctx, filtersCtxKey+contextKey(path), filters)
}

// PageFiltersFromCtx retrieves the current page display filters from the context.
func PageFiltersFromCtx(ctx context.Context, path string) *ListDisplayFilters {
	filters, found := ctx.Value(filtersCtxKey + contextKey(path)).(*ListDisplayFilters)
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

// SearchRequestToCtx stores a SearchRequest in the context. This is used to update the search filters dialog in the
// page header as appropriate.
func SearchRequestToCtx(ctx context.Context, request SearchRequest) context.Context {
	return context.WithValue(ctx, searchRequestCtxKey, request)
}

// SearchRequestFromCtx retrieves a SearchRequest from the context. This is used to update the search filters dialog in the
// page header as appropriate.
func SearchRequestFromCtx(ctx context.Context) *SearchRequest {
	request, found := ctx.Value(searchRequestCtxKey).(SearchRequest)
	if !found {
		return NewSearchRequest()
	}
	return &request
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

func DataAPIToCtx(ctx context.Context, api DataAPI) context.Context {
	return context.WithValue(ctx, dataAPICtxKey, api)
}

func DataAPIFromCtx(ctx context.Context) DataAPI {
	api, found := ctx.Value(dataAPICtxKey).(DataAPI)
	if !found {
		return nil
	}
	return api
}
