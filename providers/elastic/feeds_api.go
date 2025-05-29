// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic/aggregations"
	"github.com/joshuar/go-feed-me/providers/elastic/bulk"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
)

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

// GetFeed retrieves the feed with the given id.
func (e *API) GetFeed(ctx context.Context, id models.FeedID) (*models.Feed, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrGetFailed, ErrFetchCtx)
	}

	resp, err := NewGetRequest(e.GetAPI(), index, id).Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrGetFailed, err)
	}

	// Stop if there are no hits
	if !resp.Found {
		return nil, errors.Join(ErrGetFailed, errors.New("no job state"))
	}

	// Loop through this set of results.
	state, err := ExtractSource[*models.Feed](resp.Source_)
	if err != nil {
		return nil, errors.Join(ErrGetFailed, err)
	}

	return state, nil
}

func (e *API) GetAllFeeds(ctx context.Context, feedIDs ...models.FeedID) (models.Feeds, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	resp, err := NewMGetRequest(e.GetAPI(),
		GetFromIndex(index),
		GetIDs(feedIDs...)).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrGetFailed, err)
	}
	feeds, warnings := ExtractSourceFromDocs[*models.Feed](resp.Docs)
	if warnings != nil {
		slogctx.FromCtx(ctx).Warn("Problems occurred while extracting source from docs.",
			slog.Any("warnings", warnings))
	}
	return feeds, nil
}

// FeedsSearchAll will retrieve all feeds matching the given queries, sorted by the given sort. It paginates through the
// entire feeds index.
func (a *API) FeedsSearchAll(ctx context.Context, queries ...query.Option) (models.Feeds, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	var results models.Feeds

	searchSize := 100
	pagination := make([]types.FieldValue, 0)

	for {
		var (
			feeds    models.Feeds
			warnings error
		)

		resp, err := NewSearchRequest(a.API,
			WithSearchIndex(index),
			WithSearchQueryOptions(queries...),
			WithSearchSize(searchSize),
			WithSearchAfter(pagination),
			WithSortOptions(SortByDocID("feed_id")),
		).Do(ctx)
		if err != nil {
			return nil, errors.Join(ErrSearchFailed, err)
		}

		feeds, pagination, warnings = ExtractSourceFromHits[*models.Feed](resp.Hits.Hits)
		if warnings != nil {
			slogctx.FromCtx(ctx).Warn("Problems occurred while extracting source from docs.",
				slog.Any("warnings", err))
		}

		results = append(results, feeds...)

		// Stop if we are at the end of the results.
		if int(resp.Hits.Total.Value) < searchSize {
			break
		}
	}

	return results, nil
}

// ItemsSearch performs a search query on feed items with the given query
// options. It returns the raw search response.
func (e *API) ItemsSearch(ctx context.Context, query query.Option, filters models.Filters, pagination models.Pagination) (*search.Response, error) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	var sortValues []types.FieldValue
	if pagination != "" {
		var err error
		sortValues, err = decodePagination(pagination)
		if err != nil {
			return nil, errors.Join(ErrSearchFailed, err)
		}
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
func (e *API) ItemsAggregation(ctx context.Context, query query.Option, aggregations ...aggregations.Aggregation) (*search.Response, error) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	req := NewSearchRequest(e.GetAPI(),
		WithSearchIndex(index),
		WithSearchQueryOptions(query),
		WithSearchSize(0),
		WithAggregations(aggregations...),
		WithSortOptions(setItemSort(models.NewFilters().Sort())),
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

// setFeedSort will define how Feeds are sorted based on the given sort options (from filters).
func setFeedSort(sort models.Sort) (FieldSort, FieldSort) {
	// Adjust based on given sort options.
	switch sort.SortBy {
	case models.SortByLastUpdated:
		switch sort.SortOrder {
		case models.SortOrderAsc:
			return NewFieldSort("updated", models.SortOrderAsc), NewFieldSort("feed_id", models.SortOrderDesc)
		case models.SortOrderDesc:
			return NewFieldSort("updated", models.SortOrderDesc), NewFieldSort("feed_id", models.SortOrderDesc)
		}
	}
	return NewFieldSort("updated", models.SortOrderDesc), NewFieldSort("feed_id", models.SortOrderDesc)
}

// setItemSort will define how Items are sorted based on the given sort options (from filters).
func setItemSort(sort models.Sort) (FieldSort, FieldSort) {
	// Adjust based on given sort options.
	switch sort.SortBy {
	case models.SortByLastUpdated:
		switch sort.SortOrder {
		case models.SortOrderAsc:
			return NewFieldSort("updated", models.SortOrderAsc), NewFieldSort("item_id", models.SortOrderDesc)
		case models.SortOrderDesc:
			return NewFieldSort("updated", models.SortOrderDesc), NewFieldSort("item_id", models.SortOrderDesc)
		}
	}
	return NewFieldSort("updated", models.SortOrderDesc), NewFieldSort("item_id", models.SortOrderDesc)
}
