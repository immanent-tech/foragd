// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package handlers contains chainable handlers/middleware for routing.
package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic/bulk"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
)

var (
	// ErrInvalidUser indicates the user data is invalid. This might be the case if the retrieved session is corrupted.
	ErrInvalidUser = errors.New("user data is invalid")
	// ErrMissingRequestData indicates data was expected in the request (usually in the context) but was not found.
	ErrMissingRequestData = errors.New("request data is missing")
)

// Keys for objects stored within the context and passed between handlers.
const (
	subscriptionRequestsCtxKey contextKey = "subscriptionRequests"
	subscriptionsCtxKey        contextKey = "subscriptions"
	feedsCtxKey                contextKey = "feeds"
)

// Keys for objects stored within the session.
const (
	feedFiltersSessionKey = "feed_filters"
	itemFiltersSessionKey = "item_filters"
	HomeHistorySessionKey = "home_history"
)

type contextKey string

// DataAPI represents the API surface for interacting with the database/datastore backend.
type DataAPI interface {
	// User methods:
	AddUser(ctx context.Context, userID models.UserID) error
	GetUser(ctx context.Context, userID models.UserID) (*models.User, error)
	// Subscription methods:
	GetSubscription(ctx context.Context, subscriptionID models.SubscriptionID) (*models.Subscription, error)
	GetSubscriptions(ctx context.Context) (models.Subscriptions, models.Pagination, error)
	MarkSubscriptions(ctx context.Context, mark models.Mark, subscriptionIDs ...models.SubscriptionID) error
	AddSubscriptions(ctx context.Context, subscriptions models.Subscriptions) error
	EditSubscription(ctx context.Context, subscriptionID models.SubscriptionID, edits *models.SubscriptionCustomisation) error
	RemoveSubscriptions(ctx context.Context, subscriptionIDs ...models.SubscriptionID) error
	// Feeds methods:
	// GetFeedsByURL(ctx context.Context, urls ...models.URL) (models.Feeds, error)
	FeedsSearchAll(ctx context.Context, queries ...query.Option) (models.Feeds, error)
	AddFeeds(ctx context.Context, feeds ...*models.Feed) (*bulk.Response, error)
	// Item methods:
	GetItem(ctx context.Context, feedID models.FeedID, itemID models.ItemID) (*models.Item, bool, error)
	GetItems(ctx context.Context) (models.Items, models.Pagination, error)
	MarkItems(ctx context.Context, marks ...*models.MarkFeedItems) error
}

// AuthAPI represents the API surface for interacting with the auth backend.
type AuthAPI interface {
	GetAuthURL(req *http.Request) (string, error)
	CompleteUserAuth(res http.ResponseWriter, req *http.Request) error
	GetUserID(ctx context.Context) models.UserID
}

// SessionAPI represents the API surface for interacting with the session backend.
type SessionAPI interface {
	Put(ctx context.Context, key string, value any)
	Get(ctx context.Context, key string) any
}

func GenerateBacklink(ctx context.Context, sessionAPI SessionAPI, currentRoute string) *models.Route {
	switch {
	case strings.HasPrefix(currentRoute, models.FeedsRoute):
		return models.NewRoute("/home", nil)
	case strings.HasPrefix(currentRoute, models.ItemsRoute):
		feedFilters, ok := sessionAPI.Get(ctx, feedFiltersSessionKey).(models.Filters)
		if !ok {
			feedFilters = *models.NewFilters()
		}
		return models.NewRoute(models.FeedsRoute, &feedFilters)
	case strings.Contains(currentRoute, "feed_") && strings.Contains(currentRoute, "item_"):
		itemFilters, ok := sessionAPI.Get(ctx, itemFiltersSessionKey).(models.Filters)
		if !ok {
			itemFilters = *models.NewFilters()
		}
		return models.NewRoute(models.ItemsRoute, &itemFilters)
	default:
		return nil
	}
}
