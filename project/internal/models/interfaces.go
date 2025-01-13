// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
)

// AuthAPI contains methods for handling auth requests.
type AuthAPI interface {
	Create(ctx context.Context, user *UserSignup) (string, error)
}

// UserActionsAPI contains methods for handling user requests.
type UserActionsAPI interface {
	UserActionAddSubscriptions(ctx context.Context, subscriptions ...SubscriptionRequest) error
	UseActionrMarkItemsRead(ctx context.Context, items ...APIReadItem) error
	UserActionGetItem(ctx context.Context, feedID FeedID, itemID ItemID) (APIItem, bool, error)
	UserActionGetItems(ctx context.Context, filters APISearchFilters) ([]APIItem, []byte, error)
	UserActionGetFeeds(ctx context.Context, filters APISearchFilters) ([]APIFeed, error)
	UserActionCountUnread(ctx context.Context, feedIDs ...FeedID) (int, error)
}

// UserManagementAPI contains methods for user management.
type UserManagementAPI interface {
	UserExists(ctx context.Context, userID UserID) (bool, error)
	GetUser(ctx context.Context) (User, error)
	AddUser(ctx context.Context, userID UserID) error
}

// FeedManagementAPI contains methods for feed/item management.
type FeedManagementAPI interface {
	GetFeedByURL(ctx context.Context, url URL) (APIFeed, error)
	AddFeeds(ctx context.Context, feeds ...Feed) error
}

// SessionManagementAPI contains methods for session management.
type SessionManagementAPI interface {
	GetTokens(ctx context.Context) (*Tokens, error)
}
