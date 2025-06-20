// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"

	"github.com/joshuar/go-feed-me/providers/elastic/aggregations"
	"github.com/joshuar/go-feed-me/providers/elastic/bulk"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
)

var ErrBackend = errors.New("backend API error")

// SubscriptionsAPI contains methods for fetching/manipulating subscription data.
type SubscriptionsAPI interface {
	GetSubscriptionCustomisations(ctx context.Context, subscriptionIDs ...SubscriptionID) (SubscriptionCustomisations, error)
	AddSubscriptionCustomisations(ctx context.Context, customisations ...*SubscriptionCustomisation) (map[SubscriptionID]*bulk.OperationResponse, error)
	SearchSubscriptionCustomisations(ctx context.Context, query query.Option, count int, sort *Sort, pagination *Pagination) (SubscriptionCustomisations, Pagination, error)
	UpdateSubscriptionCustomisation(ctx context.Context, edits *SubscriptionEdit) error
	DeleteSubscriptionCustomisations(ctx context.Context, subscriptionIDs ...SubscriptionID) error
}

// FeedsAPI contains methods for manipulating feed data.
type FeedsAPI interface {
	GetFeeds(ctx context.Context, feedIDs ...FeedID) (Feeds, error)
	AddFeeds(ctx context.Context, feeds ...*Feed) (map[FeedID]*bulk.OperationResponse, error)
	SearchFeeds(ctx context.Context, query query.Option, count int, sort *Sort, pagination *Pagination) (Feeds, Pagination, error)
}

// ItemsAPI contains methods for manipulating item data.
type ItemsAPI interface {
	SearchItems(ctx context.Context, query query.Option, count int, sort *Sort, pagination *Pagination) (Items, Pagination, error)
	ItemsAggregation(ctx context.Context, query query.Option, aggregations ...aggregations.Aggregation) (*search.Response, *Response)
}

// UserAPI contains methods for manipulating user data.
type UserAPI interface {
	AddUser(ctx context.Context, userID UserID) error
	GetUser(ctx context.Context, userID UserID) (*User, error)
	UpdateUser(ctx context.Context, partialUpdate map[string]any) *Response
}

// DocumentsAPI contains methods for fetching manipulating any type of document data.
type DocumentsAPI interface {
	SubscriptionsAPI
	FeedsAPI
	ItemsAPI
	UserAPI
	MultiSearch(ctx context.Context, feedsQuery, itemsQuery *query.MSearchOptions) (Feeds, Items, error)
}

// UserBackendAPI contains the methods for creating users on an auth backend.
type UserBackendAPI interface {
	Create(ctx context.Context, details *UserSignupRequest) (string, error)
}
