// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"net/http"

	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"

	"github.com/immanent-tech/foragd/providers/elastic/aggregations"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/results"
)

var ErrBackend = errors.New("backend API error")

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
	FeedsAPI
	ItemsAPI
	UserAPI
	FindSuggestions(ctx context.Context, searchTerms string) (results.MSearchResults, error)
	CountAllUnread(ctx context.Context) (int64, error)
}

// UserBackendAPI contains the methods for creating users on an auth backend.
type UserBackendAPI interface {
	Create(ctx context.Context, details *UserSignupRequest) (string, error)
}

type AuthAPI interface {
	GetAuthURL(req *http.Request) (string, error)
	CompleteUserAuth(res http.ResponseWriter, req *http.Request) error
	GetUserID(ctx context.Context) UserID
	SetProviderName(ctx context.Context, provider string)
}
