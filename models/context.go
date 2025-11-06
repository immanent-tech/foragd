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
)

type contextKey string

// UserToCtx stores a user in the context.
func UserToCtx(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userCtxKey, user)
}

// UserFromCtx retrieves a user from the context. If no user was found, a non-nil error will be returned.
func UserFromCtx(ctx context.Context) (*User, error) {
	user, found := ctx.Value(userCtxKey).(*User)
	if !found {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func CSRFTokenToCtx(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, csrfTokenCtxKey, token)
}

func CSRFTokenFromCtx(ctx context.Context) string {
	if token, ok := ctx.Value(csrfTokenCtxKey).(string); ok {
		return token
	}
	return ""
}

func PageFiltersToCtx(ctx context.Context, path string, filters Filters) context.Context {
	return context.WithValue(ctx, filtersCtxKey+contextKey(path), filters)
}

func PageFiltersFromCtx(ctx context.Context, path string) *ListDisplayFilters {
	filters, found := ctx.Value(filtersCtxKey + contextKey(path)).(*ListDisplayFilters)
	if !found {
		newFilters := NewListDisplayFilters()
		return &newFilters
	}
	return filters
}

func PathToCtx(ctx context.Context, path string) context.Context {
	return context.WithValue(ctx, pathCtxKey, path)
}

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
