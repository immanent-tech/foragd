// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/schema"
)

// GetFeed retrieves a single feed with the given ID.
func (a *API) GetFeed(ctx context.Context, id models.FeedID) (*models.Feed, error) {
	index, err := FeedsReadIndexFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("get feed: %w", ErrNoIndexInCtx)
	}
	feed, err := GetDoc[models.FeedID, *models.Feed](ctx, a.GetAPI(), index, id)
	if err != nil {
		return nil, fmt.Errorf("get feed: %w", err)
	}
	return feed, nil
}

// CreateFeed creates a new feed doc in Elasticsearch.
func (a *API) CreateFeed(ctx context.Context, feed *models.Feed) error {
	index, err := FeedsWriteIndexFromCtx(ctx)
	if err != nil {
		return fmt.Errorf("create feed: %w", ErrNoIndexInCtx)
	}
	err = CreateDoc(ctx, a.GetAPI(), index, feed.GetID(), feed)
	if err != nil {
		return fmt.Errorf("create feed: %w", err)
	}
	return nil
}

// DeleteFeed deletes a feed doc with the given ID from Elasticsearch.
func (a *API) DeleteFeed(ctx context.Context, id models.FeedID) error {
	index, err := FeedsWriteIndexFromCtx(ctx)
	if err != nil {
		return fmt.Errorf("delete feed: %w", ErrNoIndexInCtx)
	}
	// Delete the feed.
	err = DeleteDoc(ctx, a.GetAPI(), index, id)
	if err != nil {
		return fmt.Errorf("delete feed: %w", err)
	}
	return nil
}

// GetFeeds retrieves the feeds with the given IDs.
func (a *API) GetFeeds(ctx context.Context, ids ...models.FeedID) (models.Feeds, error) {
	index, err := FeedsReadIndexFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("get feeds: %w", ErrNoIndexInCtx)
	}

	feeds, err := GetDocs[models.FeedID, *models.Feed](ctx, a.GetAPI(), index, ids...)
	if err != nil {
		return nil, fmt.Errorf("get feeds: %w", err)
	}
	return feeds, nil
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
	index, err := FeedsReadIndexFromCtx(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("search feeds: %w", ErrNoIndexInCtx)
	}

	searchAfter, err := decodePagination(pagination)
	if err != nil {
		return nil, "", models.NewAPIError( //nolint:wrapcheck
			fmt.Errorf("search feeds: decode pagination failed: %w", err),
			http.StatusInternalServerError,
		)
	}

	// Perform search.
	feeds, newSearchAfter, err := Search[*models.Feed](ctx, a.GetAPI(), index, query, count,
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
			return nil, "", models.NewAPIError( //nolint:wrapcheck
				fmt.Errorf("search feeds: encode pagination failed: %w", err),
				http.StatusInternalServerError,
			)
		}
		return feeds, *pagination, nil
	}

	return feeds, "", nil
}

// GetNewFeeds will return a slice of all feeds that have been created but not fetched.
func (a *API) GetNewFeeds(ctx context.Context) (models.Feeds, error) {
	index := schema.FeedsIndexPrefix + schema.IndexReadSuffix

	var (
		feeds models.Feeds
		err   error
	)

	//  We detect new feeds by those where the last_fetched value equals Unix Epoch, indicating they
	// don't have a job scheduled for updating their items.
	feeds, err = SearchAll[*models.Feed](
		ctx,
		a.GetAPI(),
		index,
		query.Term("last_fetched", models.UnixEpoch),
		defaultPaginationSize,
	)
	if err != nil {
		return nil, fmt.Errorf("get new feeds since: %w", err)
	}
	return feeds, nil
}

// UpdateFeedLastFetched will update the feed with the given id, using the new feed information provided.
func (a *API) UpdateFeedLastFetched(ctx context.Context, id models.FeedID, timestamp time.Time) error {
	index := schema.FeedsIndexPrefix + schema.IndexWriteSuffix

	updates := map[string]any{
		"last_fetched": timestamp,
	}

	if err := UpdateDoc(ctx, a.GetAPI(), index, id, updates); err != nil {
		return fmt.Errorf("update feed: %w", err)
	}
	return nil
}
