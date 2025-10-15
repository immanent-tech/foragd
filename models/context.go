// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
)

const (
	userCtxKey        contextKey = "user"
	listFiltersCtxKey contextKey = "list_filters"
	csrfTokenCtxKey   contextKey = "csrfToken"
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

func ListFiltersToCtx(ctx context.Context, filters ListDisplayFilters) context.Context {
	return context.WithValue(ctx, listFiltersCtxKey, filters)
}

func ListFiltersFromCtx(ctx context.Context) ListDisplayFilters {
	filters, found := ctx.Value(listFiltersCtxKey).(ListDisplayFilters)
	if !found {
		return NewListDisplayFilters()
	}
	return filters
}
