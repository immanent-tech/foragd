// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/joshuar/go-feed-me/internal/api"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/bulk"
)

type ElasticAPI struct {
	*typedapi.API
	logger *slog.Logger
}

func (a *ElasticAPI) Log() *slog.Logger {
	return a.logger
}

// NewRequest creates a new bulk requesst object with the given options.
// After creation, document operations can be added with the AddOperations method.
func (a *ElasticAPI) NewBulkRequest(ctx context.Context, options ...bulk.BulkOption) (chan bulk.BulkOperation, chan error) {
	req := &bulk.BulkRequest{
		Bulk: a.Bulk(),
	}

	for _, option := range options {
		option(req)
	}

	bulkOps := make(chan bulk.BulkOperation)
	errorCh := make(chan error)

	go func() {
		defer close(errorCh)

		for op := range bulkOps {
			if err := req.AddOperation(op); err != nil {
				logging.FromContext(ctx).Warn("Could not add operation to bulk request.",
					slog.Any("error", err))
			}
		}

		resp, err := req.Do(ctx)
		// Handle response.
		switch {
		case err != nil:
			errorCh <- fmt.Errorf("bulk index failed: %w", err)
		case resp.Errors:
			errorCh <- fmt.Errorf("bulk index completed with some errors: %w", resp.Items)
		default:
			errorCh <- nil
		}
	}()

	return bulkOps, errorCh
}

// AddItems will bulk index the given items.
func (a *ElasticAPI) AddItems(ctx context.Context, items ...models.Item) error {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return ErrNoIndexInCtx
	}

	bulkOps, err := a.NewBulkRequest(ctx)

	go func() {
		defer close(bulkOps)

		for _, item := range items {
			logging.FromContext(ctx).Debug("Adding item",
				slog.String("name", item.Title),
				slog.String("item_id", item.ID),
				slog.String("feed_id", item.FeedID),
			)

			bulkOps <- bulk.NewOp(&item,
				bulk.SetDocID(item.ID),
				bulk.ToIndex(index),
			)
		}
	}()

	return <-err
}

func (a *ElasticAPI) AddFeeds(ctx context.Context, feeds ...models.Feed) error {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return ErrNoIndexInCtx
	}

	bulkOps, err := a.NewBulkRequest(ctx)

	go func() {
		defer close(bulkOps)

		for _, feed := range feeds {
			feed.Items = nil // don't index items in feed.
			logging.FromContext(ctx).Debug("Adding feed",
				slog.String("name", feed.Title),
				slog.String("item_id", feed.ID),
			)

			bulkOps <- bulk.NewOp(&feed,
				bulk.SetDocID(feed.ID),
				bulk.ToIndex(index),
			)
		}
	}()

	return <-err
}

func (a *ElasticAPI) GetFeedJobState(ctx context.Context, feedID models.FeedID) (*api.FeedState, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrGetFailed, ErrNoIndexInCtx)
	}

	resp, err := NewGetRequest(a.API, index, feedID).Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrGetFailed, err)
	}

	// Stop if there are no hits
	if !resp.Found {
		return nil, errors.Join(ErrGetFailed, errors.New("no job state"))
	}

	// Loop through this set of results.
	state, err := ExtractSource[api.FeedState](resp.Source_)
	if err != nil {
		return nil, errors.Join(ErrGetFailed, err)
	}

	return &state, nil
}

// UpdateFeedJobState will update the job for a feed in the scheduler jobs
// index. Specifically, it will update the last_fetched value indicating when
// the feed last fetched its items.
func (a *ElasticAPI) UpdateFeedJobState(ctx context.Context, state *api.FeedState) error {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return errors.Join(ErrGetFailed, ErrNoIndexInCtx)
	}

	if state.UpdatedAt.IsZero() {
		updated := time.Now().UTC()
		state.UpdatedAt = &updated
	}

	if _, err := NewDocUpdateRequest(a.API, index, state.ID,
		WithPartialDocUpdate(map[string]any{
			"updated_at": state.UpdatedAt,
		}),
	).Do(ctx); err != nil {
		return errors.Join(ErrUpdateFailed, err)
	}

	return nil
}

// GetNewFeedsSince retrieves a list of feeds that have been updated since the
// given time.
func (a *ElasticAPI) GetNewFeedsSince(ctx context.Context, since time.Time) ([]models.APIFeed, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrNoIndexInCtx)
	}

	logging.FromContext(ctx).Debug("Finding new feeds.",
		slog.Time("since", since))

	var newFeeds []models.APIFeed

	searchSize := 100
	pagination := make([]types.FieldValue, 0)

	for {
		var (
			feeds    []models.APIFeed
			warnings error
		)

		resp, err := NewSearchRequest(a.API,
			WithSearchIndex(index),
			WithSearchQueryOptions(QuerySince("created_at", since)),
			WithSearchSize(searchSize),
			WithSearchAfter(pagination),
			WithSortOptions(SortByDocID("feed_id")),
		).Do(ctx)
		if err != nil {
			return nil, errors.Join(ErrSearchFailed, err)
		}

		feeds, pagination, warnings = ExtractSourceFromHits[models.APIFeed](resp.Hits.Hits)
		if warnings != nil {
			logging.FromContext(ctx).Warn("Problems occurred while extracting source from docs.",
				slog.Any("warnings", err))
		}

		newFeeds = append(newFeeds, feeds...)

		// Stop if we are at the end of the results.
		if int(resp.Hits.Total.Value) < searchSize {
			break
		}
	}

	return newFeeds, nil
}
