// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
)

const (
	userCtxKey contextKey = "user"
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
