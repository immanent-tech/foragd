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
	FindSubscriptionByFeedID(ctx context.Context, feedID string) (*Subscription, error)
	AddSubscription(ctx context.Context, userID string, sub *Subscription) error
	GetAllSubscriptions(ctx context.Context) ([]Subscription, error)
	GetUserByID(ctx context.Context, userID string) (*User, error)
	AddUser(ctx context.Context, userID string, newUser *APIUser) error
}

type Cache interface {
	GetFeedByURL(ctx context.Context, url string) (APIFeed, error)
	AddFeeds(ctx context.Context, feeds ...Feed) error
	GetFeeds(ctx context.Context, feedIDs ...string) ([]APIFeed, error)
	GetFeedItems(ctx context.Context, feedIDs ...string) ([]APIItem, error)
}

type Session interface {
	GetTokens(ctx context.Context) (*Tokens, error)
}
