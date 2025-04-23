// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/sortorder"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic/bulk"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
)

// feedSortFields defines the fields for sorting feeds.
type feedSortFields struct {
	Updated types.FieldSort `json:"updated"`
	FeedID  types.FieldSort `json:"feed_id"`
}

// itemSortFields defines the fields for sorting items.
type itemSortFields struct {
	Timestamp types.FieldSort `json:"@timestamp"`
	ItemID    types.FieldSort `json:"item_id"`
}

var defaultDatetimeFormat = "strict_date_optional_time_nanos"

// AddItems will bulk index the given items.
func (e *API) AddItems(ctx context.Context, items ...*models.Item) (*bulk.Response, error) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, ErrFetchCtx
	}

	bulkOps, respCh := bulk.NewRequest(ctx, e)

	go func() {
		defer close(bulkOps)

		for _, item := range items {
			slogctx.FromCtx(ctx).Debug("Adding item",
				slog.String("name", item.GetTitle()),
				slog.String("item_id", item.GetID()),
				slog.String("feed_id", item.GetFeedID()),
			)

			bulkOps <- bulk.NewOperation(&item,
				bulk.SetDocID(item.GetID()),
				bulk.ToIndex(index),
			)
		}
	}()

	resp := <-respCh

	return &resp, nil
}

// AddFeeds will bulk index the given feeds.
func (e *API) AddFeeds(ctx context.Context, feeds ...*models.Feed) (*bulk.Response, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, ErrFetchCtx
	}

	bulkOps, respCh := bulk.NewRequest(ctx, e)

	go func() {
		defer close(bulkOps)

		for _, feed := range feeds {
			slogctx.FromCtx(ctx).Debug("Adding feed",
				slog.String("name", feed.GetTitle()),
				slog.String("feed_id", feed.GetID()),
			)

			bulkOps <- bulk.NewOperation(&feed,
				bulk.SetDocID(feed.GetID()),
				bulk.ToIndex(index),
			)
		}
	}()

	resp := <-respCh

	return &resp, nil
}

// GetFeedsByURL retrieves a list of APIFeeds based on the given URLs.
func (e *API) GetFeedsByURL(ctx context.Context, urls ...models.URL) (models.Feeds, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, ErrFetchCtx
	}

	feeds := make([]*models.Feed, 0, len(urls))

	resp, err := NewSearchRequest(e.GetAPI(),
		WithSearchIndex(index),
		WithFields("feed_id", "source"),
		WithSearchQueryOptions(query.URLs("source", urls...)),
		WithSearchSize(len(urls)),
		WithSortOptions(SortByDocID("feed_id")),
	).Do(ctx)
	if err != nil {
		return nil, models.NewMessage("Fetching feeds failed.", models.MessageStatusError, models.WithError(err))
	}
	// Stop if there are no hits
	if len(resp.Hits.Hits) == 0 {
		return nil, nil
	}
	// Loop through this set of results.
	sources, _, warnings := ExtractSourceFromHits[*models.Feed](resp.Hits.Hits)
	if warnings != nil {
		slogctx.FromCtx(ctx).Warn("Problems occurred while extracting source from docs.",
			slog.Any("warnings", err))
	}

	feeds = append(feeds, sources...)

	return feeds, nil
}

// FeedsSearch searches the feeds index for feeds matching the relevant filters.
func (e *API) FeedsSearch(ctx context.Context, filters models.Filters, pagination models.Pagination) (models.Feeds, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	sortValues, err := decodePagination(pagination)
	if err != nil {
		return nil, errors.Join(ErrSearchFailed, err)
	}

	resp, err := NewSearchRequest(e.GetAPI(),
		WithSearchIndex(index),
		WithSearchQueryOptions(
			query.Bool(
				// Match either the FeedID OR the Category.
				query.Should(
					query.FeedIDs(filters.Feeds...),
					query.Categories(filters.Categories...),
				),
			),
		),
		WithSearchSize(filters.Count),
		WithSearchAfter(sortValues),
		WithSortOptions(setFeedSort(filters.Sort())),
	).Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrSearchFailed, err)
	}
	// Stop if there are no hits
	if len(resp.Hits.Hits) == 0 {
		return nil, nil
	}
	// Loop through this set of results.
	sources, sort, warnings := ExtractSourceFromHits[*models.Feed](resp.Hits.Hits)
	if warnings != nil {
		slogctx.FromCtx(ctx).Warn("Problems occurred while extracting source from docs.",
			slog.Any("warnings", err))
	}
	spew.Dump(sort)

	slogctx.FromCtx(ctx).Debug("Searched feeds.",
		slog.Int64("hits", resp.Hits.Total.Value))

	return sources, nil
}

// ItemsSearch performs a search query on feed items with the given query
// options. It returns the raw search response.
func (e *API) ItemsSearch(ctx context.Context, query query.Option, filters models.Filters) (*search.Response, error) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	sortValues, err := decodePagination(filters.Pagination)
	if err != nil {
		return nil, errors.Join(ErrSearchFailed, err)
	}

	resp, err := NewSearchRequest(e.GetAPI(),
		WithSearchIndex(index),
		WithSearchQueryOptions(query),
		WithSearchAfter(sortValues),
		WithSearchSize(filters.Count),
		WithSortOptions(setItemSort(filters.Sort())),
	).Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrSearchFailed, err)
	}

	slogctx.FromCtx(ctx).Debug("Searched items.",
		slog.Int64("hits", resp.Hits.Total.Value))

	return resp, nil
}

// ItemsAggregation performs a search aggregation (i.e., only aggregations returned) on feed items with the given query
// options. It returns the raw search response.
func (e *API) ItemsAggregation(ctx context.Context, query query.Option, aggregation Aggregation) (*search.Response, error) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	req := NewSearchRequest(e.GetAPI(),
		WithSearchIndex(index),
		WithSearchQueryOptions(query),
		WithSearchSize(0),
		WithAggregations(aggregation),
		WithSortOptions(defaultItemSort()),
	)

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrUserActionFailed, err)
	}

	return resp, nil
}

// MarkFeedUpdated updates the timestamp indicating when the feed was last updated (i.e., new items found and indexed).
func (e *API) MarkFeedUpdated(ctx context.Context, feedID models.FeedID) error {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return errors.Join(ErrUpdateFailed, ErrFetchCtx)
	}
	partialUpdate := make(map[string]any)
	// Updated the `updated` timestamp.
	partialUpdate["updated"] = time.Now().UTC()
	// Update the user in the store with the new list of read items.
	resp, err := NewDocUpdateRequest(e.GetAPI(), index, feedID,
		WithPartialDocUpdate(partialUpdate),
	).Do(ctx)
	if err != nil {
		return errors.Join(ErrUpdateFailed, err)
	}
	slogctx.FromCtx(ctx).Debug("Updated feed.",
		slog.String("result", resp.Result.String()),
		slog.Int64("version", resp.Version_))
	return nil
}

// defaultItemSort sorts Items by updated or published date descending.
func defaultItemSort() map[string]types.FieldSort {
	return map[string]types.FieldSort{
		"@timestamp": {Order: &sortorder.Desc, Format: &defaultDatetimeFormat},
		"item_id":    {Order: &sortorder.Desc},
	}
}

// setFeedSort will define how Feeds are sorted based on the given sort options (from filters).
func setFeedSort(sort models.Sort) feedSortFields {
	// Default sort options.
	sortOptions := feedSortFields{
		Updated: types.FieldSort{Order: &sortorder.Desc, Format: &defaultDatetimeFormat},
		FeedID:  types.FieldSort{Order: &sortorder.Desc},
	}
	// Adjust based on given sort options.
	switch sort.SortBy {
	case models.SortByLastUpdated:
		switch sort.SortOrder {
		case models.SortOrderAsc:
			sortOptions.Updated.Order = &sortorder.Asc
		case models.SortOrderDesc:
			sortOptions.Updated.Order = &sortorder.Desc
		}
	}
	return sortOptions
}

// setItemSort will define how Items are sorted based on the given sort options (from filters).
func setItemSort(sort models.Sort) itemSortFields {
	// Default sort options.
	sortOptions := itemSortFields{
		Timestamp: types.FieldSort{Order: &sortorder.Desc, Format: &defaultDatetimeFormat},
		ItemID:    types.FieldSort{Order: &sortorder.Desc},
	}
	// Adjust based on given sort options.
	switch sort.SortBy {
	case models.SortByLastUpdated:
		switch sort.SortOrder {
		case models.SortOrderAsc:
			sortOptions.Timestamp.Order = &sortorder.Asc
		case models.SortOrderDesc:
			sortOptions.Timestamp.Order = &sortorder.Desc
		}
	}
	return sortOptions
}
