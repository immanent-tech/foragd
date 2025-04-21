// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package handlers contains chainable handlers/middleware for routing.
package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic/bulk"
)

var (
	// ErrInvalidUser indicates the user data is invalid. This might be the case if the retrieved session is corrupted.
	ErrInvalidUser = errors.New("user data is invalid")
	// ErrMissingRequestData indicates data was expected in the request (usually in the context) but was not found.
	ErrMissingRequestData = errors.New("request data is missing")
)

const (
	subscriptionRequestsCtxKey contextKey = "subscriptionRequests"
	subscriptionsCtxKey        contextKey = "subscriptions"
	feedsCtxKey                contextKey = "feeds"
	feedFiltersSessionKey                 = "feed_filters"
	itemFiltersSessionKey                 = "item_filters"
)

type contextKey string

type DataAPI interface {
	// User methods:
	AddUser(ctx context.Context, userID models.UserID) error
	GetUser(ctx context.Context, userID models.UserID) (*models.User, error)
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

type AuthAPI interface {
	GetAuthURL(req *http.Request) (string, error)
	CompleteUserAuth(res http.ResponseWriter, req *http.Request) error
	GetUserID(ctx context.Context) models.UserID
}

type SessionAPI interface {
	Put(ctx context.Context, key string, value any)
	Get(ctx context.Context, key string) any
}
