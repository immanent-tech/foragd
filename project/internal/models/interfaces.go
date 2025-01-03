// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
)

type Auth interface {
	Create(ctx context.Context, newUser *APIUser) (string, error)
}

type DB interface {
	IsSubscribed(ctx context.Context, feedID string) (bool, error)
	AddSubscription(ctx context.Context, userID string, sub *Subscription) error
	GetAllSubscriptions(ctx context.Context) ([]Subscription, error)
	FilterSubscriptionsByFeedID(ctx context.Context, feedIDs ...string) ([]Subscription, error)
	GetUserByID(ctx context.Context, userID string) (*User, error)
	AddUser(ctx context.Context, userID string, newUser *APIUser) error
}

type Cache interface {
	GetFeedByURL(ctx context.Context, url string) (APIFeed, error)
	AddFeeds(ctx context.Context, feeds ...Feed) error
	GetFeeds(ctx context.Context, filters APISearchFilters) ([]APIFeed, error)
	GetItems(ctx context.Context, filters APISearchFilters) ([]APIItem, []byte, error)
	MarkFeedsRead(ctx context.Context, feedIDs ...FeedID) error
	MarkItemsRead(ctx context.Context, items ...APIReadItem) error
	GetItem(ctx context.Context, feedID, itemID string) (APIItem, error)
	CountUnread(ctx context.Context, feedIDs ...string) (int, error)
}

type Session interface {
	GetTokens(ctx context.Context) (*Tokens, error)
}
