// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"errors"

	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/bulk"
)

const (
	subscriptionRequestsCtxKey contextKey = "subscriptionRequests"
	subscriptionsCtxKey        contextKey = "subscriptions"
	feedsCtxKey                contextKey = "feeds"
)

type contextKey string

// ErrInvalidUser indicates the user data is invalid. This might be the case if the retrieved session is corrupted.
var ErrInvalidUser = errors.New("user data is invalid")

type DataAPI interface {
	// User methods:
	AddUser(ctx context.Context, userID models.UserID) error
	GetUser(ctx context.Context) (*models.User, error)
	// Subscription methods:
	GetSubscriptions(ctx context.Context) (models.Subscriptions, error)
	MarkSubscriptions(ctx context.Context, marks *models.MarkFeeds) error
	AddSubscriptions(ctx context.Context, subscriptions models.Subscriptions) error
	// Feeds methods:
	GetFeedsByURL(ctx context.Context, urls ...models.URL) (models.Feeds, error)
	AddFeeds(ctx context.Context, feeds ...*models.Feed) (*bulk.Response, error)
	// Item methods:
	GetItem(ctx context.Context, feedID models.FeedID, itemID models.ItemID) (*models.Item, bool, error)
	GetItems(ctx context.Context) (models.Items, models.Pagination, error)
	MarkItems(ctx context.Context, marks ...*models.MarkFeedItems) error
}
