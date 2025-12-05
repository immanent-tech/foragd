// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"

	"github.com/immanent-tech/foragd/providers/elastic/aggregations"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	"github.com/immanent-tech/foragd/providers/elastic/query"
)

var (
	// ErrNoUserCtx indicates the user object was not found in the context.
	ErrNoUserCtx = errors.New("no valid user in context")
	// ErrInvalidAPIResult indicates that the backend API returned unexpected, invalid or an otherwise incorrect response.
	ErrInvalidAPIResult = errors.New("invalid backend API result")
)

// FeedsAPI contains API methods for Feeds.
type FeedsAPI interface {
	GetFeeds(ctx context.Context, feedIDs ...FeedID) (Feeds, error)
	GetNewFeeds(ctx context.Context) (Feeds, error)
	GetFeed(ctx context.Context, id FeedID) (*Feed, error)
	UpdateFeedLastFetched(ctx context.Context, id FeedID, timestamp time.Time) error

	SearchFeeds(
		ctx context.Context,
		query query.Option,
		count int,
		sort *Sort,
		pagination *Pagination,
	) (Feeds, Pagination, error)
	// MultiSearchFeeds(ctx context.Context, queries ...*MultiSearchQuery) (results.MSearchResults, error)
	CreateFeed(ctx context.Context, feed *Feed) error
}

// ItemsAPI contains API methods for Items.
type ItemsAPI interface {
	SearchItems(
		ctx context.Context,
		query query.Option,
		count int,
		sort *Sort,
		pagination *Pagination,
	) (Items, Pagination, error)
	AddItems(ctx context.Context, items ...*Item) (map[ItemID]*bulk.OperationResponse, error)
	CountItems(ctx context.Context, query query.Option) (int64, error)
	ItemsAggregation(
		ctx context.Context,
		query query.Option,
		count int,
		agg aggregations.Aggs,
	) (*search.Response, error)
	ArchiveArticle(ctx context.Context, article *ArticleArchive) error
	UnarchiveArticle(ctx context.Context, userID UserID, itemID ItemID) error
}

// UserAPI contains API methods for Users.
type UserAPI interface {
	CreateUser(ctx context.Context, user *User) error
	UpdateUser(ctx context.Context, id UserID, updates map[string]any) error
}

// DataAPI contains all methods for data API access.
type DataAPI interface {
	FeedsAPI
	ItemsAPI
	UserAPI
	GetAPI() *elasticsearch.TypedClient
}

// CreateFeed stores a new Feed.
func CreateFeed(ctx context.Context, dataAPI DataAPI, feed *Feed) error {
	if err := dataAPI.CreateFeed(ctx, feed); err != nil {
		return fmt.Errorf("unable to create feed: %w", err)
	}
	return nil
}

// GetArticleTopCategories performs an aggregation to return the top Item categories across the given Feeds.
func GetArticleTopCategories(ctx context.Context, dataAPI DataAPI, feeds ...FeedID) ([]Category, error) {
	// Build query.
	query := query.Bool(
		query.Filter(
			// Must match any of the given feed IDs.
			query.Terms("feed_id", feeds...),
		),
	)
	// Build aggregations.
	termsField := "categories.raw"
	termsCount := 10
	aggs := aggregations.Aggs{
		"TopCategories": types.Aggregations{
			Terms: &types.TermsAggregation{
				Field: &termsField,
				Size:  &termsCount,
			},
		},
	}
	// Perform aggregation.
	results, err := dataAPI.ItemsAggregation(ctx, query, 0, aggs)
	if err != nil {
		return nil, fmt.Errorf("unable to get top categories: %w", err)
	}

	topCategoriesAgg, ok := results.Aggregations["TopCategories"].(*types.StringTermsAggregate)
	if !ok {
		return nil, fmt.Errorf("unable to get top categories: aggregations invalid: %w", ErrInvalidAPIResult)
	}
	topCategoriesBuckets, ok := topCategoriesAgg.Buckets.([]types.StringTermsBucket)
	if !ok {
		return nil, fmt.Errorf("unable to get top categories: aggregations invalid: %w", ErrInvalidAPIResult)
	}

	topCategories := make([]Category, 0)

	for bucket := range slices.Values(topCategoriesBuckets) {
		if category, ok := bucket.Key.(Category); ok {
			topCategories = append(topCategories, category)
		}
	}

	return topCategories, nil
}

// type MultiSearchQuery struct {
// 	Name       string
// 	Index      string
// 	Query      query.Option
// 	Sort       *Sort
// 	Pagination *Pagination
// 	Size       int
// }
