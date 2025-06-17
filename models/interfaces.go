// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"errors"

	"github.com/joshuar/go-feed-me/models/feeds/types"
)

var ErrBackend = errors.New("backend API error")

// // FeedsAPI contains methods for manipulating feed/item data.
// type FeedsAPI interface {
// 	GetFeeds(ctx context.Context, feedIDs ...FeedID) (Feeds, error)
// 	SearchFeeds(ctx context.Context, query query.Option, count int, sort *Sort, pagination *Pagination) (Feeds, Pagination, error)
// 	AddFeeds(ctx context.Context, feeds ...*Feed) (*bulk.Response, error)
// 	SearchItems(ctx context.Context, query query.Option, count int, sort *Sort, pagination *Pagination) (Items, Pagination, error)
// 	ItemsAggregation(ctx context.Context, query query.Option, aggregations ...aggregations.Aggregation) (*search.Response, error)
// 	MultiSearch(ctx context.Context, feedsQuery, itemsQuery *query.MSearchOptions) (Feeds, Items, error)
// }

// // UserAPI contains methods for manipulating user data.
// type UserAPI interface {
// 	AddUser(ctx context.Context, userID UserID) error
// 	GetUser(ctx context.Context, userID UserID) (*User, error)
// 	UpdateUser(ctx context.Context, id UserID, partialUpdate map[string]any) *Response
// }

// // BackendAPI contains the feed/user apis.
// type BackendAPI interface {
// 	FeedsAPI
// 	UserAPI
// }

// Source represents a single source of data. This might be an individual feed or item.
type Source interface {
	types.ObjectCommon
	GetID() string
	GetFeedID() FeedID
	IsUnread() bool
}

// SourceWithContent is a source that has its content embedded.
type SourceWithContent interface {
	Source
	GetContent() string
}
