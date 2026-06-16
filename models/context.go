// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"net/http"
)

const (
	userCtxKey          contextKey = "user"
	subscriptionsCtxKey contextKey = "subscriptions"
	clientTypeCtxKey    contextKey = "client_type"
)

type contextKey string

var ErrCtxValueNotFound = &APIError{
	InternalError: errors.New("context value not found"),
	StatusCode:    http.StatusInternalServerError,
}

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

// ClientTypeFromCtx stores a ClientType in the context.
func ClientTypeToCtx(ctx context.Context, clientType ClientType) context.Context {
	return context.WithValue(ctx, clientTypeCtxKey, clientType)
}

// ClientTypeFromCtx retrieves the ClientType from the context, if any.
func ClientTypeFromCtx(ctx context.Context) ClientType {
	clientType, found := ctx.Value(clientTypeCtxKey).(ClientType)
	if !found {
		// Assume web client as default.
		return ClientTypeWeb
	}
	return clientType
}
