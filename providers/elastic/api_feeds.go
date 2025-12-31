// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"fmt"

	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/schema"
)

// CreateFeed creates a new feed doc in Elasticsearch.
func (a *API) CreateFeed(ctx context.Context, feed *models.Feed) error {
	index := schema.FeedsIndexPrefix + schema.IndexWriteSuffix
	if err := CreateDoc(ctx, index, feed.GetID(), feed); err != nil {
		return fmt.Errorf("create feed: %w", err)
	}
	return nil
}

// DeleteFeed deletes a feed doc with the given ID from Elasticsearch.
func (a *API) DeleteFeed(ctx context.Context, id models.FeedID) error {
	index := schema.FeedsIndexPrefix + schema.IndexWriteSuffix
	if err := DeleteDoc(ctx, index, id); err != nil {
		return fmt.Errorf("delete feed: %w", err)
	}
	return nil
}

// SearchFeeds will search the feeds index for feed matching the given query. Count, sort and pagination values are
// optional.
func (a *API) SearchFeeds(
	ctx context.Context,
	query query.Option,
	count int,
	sort *models.Sort,
	pagination *models.Pagination,
) (models.Feeds, models.Pagination, error) {
	index := schema.FeedsIndexPrefix + schema.IndexReadSuffix

	searchAfter, err := decodePagination(pagination)
	if err != nil {
		return nil, "", ErrInvalidParams
	}

	// Perform search.
	feeds, newSearchAfter, err := Search[*models.Feed](ctx, index, query, count,
		WithSortOptions[*search.Search, SearchRequest](newFeedSortOptions(sort)...),
		WithSearchAfter[*search.Search, SearchRequest](searchAfter...),
	)
	if err != nil {
		return nil, "", fmt.Errorf("search feeds: %w", err)
	}
	// Parse search after into pagination.
	if pagination != nil {
		*pagination, err = encodePagination(newSearchAfter)
		if err != nil {
			return nil, "", ErrInvalidParams
		}
		return feeds, *pagination, nil
	}

	return feeds, "", nil
}
