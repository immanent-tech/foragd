// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
)

var ErrUserCtx = errors.New("could not fetch user details from context")

const (
	userCtxKey    contextKey = "user"
	filtersCtxKey contextKey = "filters"
)

type contextKey string

// UserToCtx stores a user in the context.
func UserToCtx(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userCtxKey, user)
}

// UserFromCtx retrieves a user from the context and a boolean indicating
// whether the user was found. If a user was found, the boolean will be true and
// the user object will be valid. If a user was not found or there was a problem
// with retrieval, the boolean will be false and an empty user object will be returned.
func UserFromCtx(ctx context.Context) (*User, bool) {
	user, found := ctx.Value(userCtxKey).(*User)
	if !found {
		return nil, false
	}
	return user, true
}

// FiltersToCtx stores the current filters in the context.
func FiltersToCtx(ctx context.Context, filters Filters) context.Context {
	return context.WithValue(ctx, filtersCtxKey, filters)
}

// FiltersFromCtx retrieves the current filters from the context. If there are no filters stored in the context, the
// default filters will be returned.
func FiltersFromCtx(ctx context.Context) Filters {
	var (
		filters Filters
		found   bool
	)
	filters, found = ctx.Value(filtersCtxKey).(Filters)
	if !found {
		filters = *NewFilters()
	}
	return filters
}
