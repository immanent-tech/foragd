// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
)

const (
	userContextKey            contextKey = "user"
	subscriptionRequestCtxKey contextKey = "subscriptionRequest"
	feedManagementAPICtxKey   contextKey = "feedManagementAPI"
	pageNavigationCtxKey      contextKey = "pageNavigation"
	itemSetBasePathCtxKey     contextKey = "itemSetBasePath"
)

type contextKey string

// UserToCtx stores a user in the context.
func UserToCtx(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// UserFromCtx retrieves a user from the context and a boolean indicating
// whether the user was found. If a user was found, the boolean will be true and
// the user object will be valid. If a user was not found or there was a problem
// with retrieval, the boolean will be false and an empty user object will be returned.
func UserFromCtx(ctx context.Context) (*User, bool) {
	user, found := ctx.Value(userContextKey).(*User)
	if !found {
		return nil, false
	}

	return user, true
}

func SubscriptionRequestToCtx(ctx context.Context, req *SubscriptionRequest) context.Context {
	return context.WithValue(ctx, subscriptionRequestCtxKey, req)
}

func SubscriptionRequestFromCtx(ctx context.Context) *SubscriptionRequest {
	req, found := ctx.Value(subscriptionRequestCtxKey).(*SubscriptionRequest)
	if !found {
		return nil
	}

	return req
}

func FeedManagementAPIToCtx(ctx context.Context, api FeedManagementAPI) context.Context {
	return context.WithValue(ctx, feedManagementAPICtxKey, api)
}

func FeedManagementAPIFromCtx(ctx context.Context) FeedManagementAPI {
	api, found := ctx.Value(feedManagementAPICtxKey).(FeedManagementAPI)
	if !found {
		return nil
	}

	return api
}

func PageNavigationToCtx(ctx context.Context, navigation *APIPageNavigation) context.Context {
	return context.WithValue(ctx, pageNavigationCtxKey, navigation)
}

func PageNavigationFromCtx(ctx context.Context) *APIPageNavigation {
	navigation, found := ctx.Value(pageNavigationCtxKey).(*APIPageNavigation)
	if !found {
		return nil
	}

	return navigation
}

func ItemSetBasePathToCtx(ctx context.Context, path string) context.Context {
	return context.WithValue(ctx, itemSetBasePathCtxKey, path)
}

func ItemSetBasePathFromCtx(ctx context.Context) string {
	path, found := ctx.Value(itemSetBasePathCtxKey).(string)
	if !found {
		return ""
	}

	return path
}
