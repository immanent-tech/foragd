// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"

	"github.com/joshuar/go-feed-me/internal/platforms/elastic/bulk"
)

var ErrBackend = errors.New("backend API error")

// // AuthAPI contains methods for handling auth requests.
// type AuthAPI interface {
// 	Create(ctx context.Context, user *api.UserSignupRequest) (string, error)
// }

// UserActionsAPI contains methods for handling user requests.
// type UserActionsAPI interface {
// 	UserActionMarkItems(ctx context.Context, mark Mark, items ...ItemID) error
// 	UserActionMarkFeeds(ctx context.Context, mark Mark, feeds ...FeedID) error
// 	UserActionGetItem(ctx context.Context, feedID FeedID, itemID ItemID) (APIItem, bool, error)
// 	UserActionGetItems(ctx context.Context, filters Filters) ([]*APIItem, Pagination, error)
// 	UserActionGetFeeds(ctx context.Context, filters Filters) ([]*APIFeed, error)
// 	// UserActionCountUnread(ctx context.Context, feedIDs ...FeedID) (int, error)
// }

// SubscriptionsAPI contains methods for handling user subscriptions.
// type SubscriptionsAPI interface {
// 	AddSubscriptions(ctx context.Context, details ...*SubscriptionRequest) error
// }

// UserManagementAPI contains methods for user management.
type UserManagementAPI interface {
	UserExists(ctx context.Context, userID UserID) (bool, error)
	GetUser(ctx context.Context) (*User, error)
	AddUser(ctx context.Context, userID UserID) error
	UpdateUser(ctx context.Context, userID UserID, update map[string]any) error
}

// type FeedJobStateAPI interface {
// 	GetFeedJobState(ctx context.Context, feedID FeedID) (*APIFeedState, error)
// 	UpdateFeedJobState(ctx context.Context, state *APIFeedState) error
// }

// FeedManagementAPI contains methods for feed/item management.
type FeedManagementAPI interface {
	GetFeedByURL(ctx context.Context, url URL) (*APIFeed, error)
	AddFeeds(ctx context.Context, feeds ...Feed) (*bulk.Response, error)
	AddItems(ctx context.Context, items ...Item) (*bulk.Response, error)
	// FeedJobStateAPI
}

// SessionManagementAPI contains methods for session management.
type SessionManagementAPI interface {
	GetTokens(ctx context.Context) (*Tokens, error)
}
