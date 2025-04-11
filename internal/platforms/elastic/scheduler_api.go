// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/query"
)

func (a *API) GetFeedJobState(ctx context.Context, feedID models.FeedID) (*models.FeedState, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrGetFailed, ErrFetchCtx)
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
	state, err := ExtractSource[models.FeedState](resp.Source_)
	if err != nil {
		return nil, errors.Join(ErrGetFailed, err)
	}

	return &state, nil
}

// UpdateFeedJobState will update the job for a feed in the scheduler jobs
// index. Specifically, it will update the last_fetched value indicating when
// the feed last fetched its items.
func (a *API) UpdateFeedJobState(ctx context.Context, state *models.FeedState) error {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return errors.Join(ErrGetFailed, ErrFetchCtx)
	}

	if state.UpdatedAt.IsZero() {
		updated := time.Now().UTC()
		state.UpdatedAt = &updated
	}

	if _, err := NewDocUpdateRequest(a.API, index, state.FeedID,
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
func (a *API) GetNewFeedsSince(ctx context.Context, since time.Time) (models.Feeds, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	slogctx.FromCtx(ctx).Debug("Finding new feeds.",
		slog.Time("since", since))

	var newFeeds models.Feeds

	searchSize := 100
	pagination := make([]types.FieldValue, 0)

	for {
		var (
			feeds    models.Feeds
			warnings error
		)

		resp, err := NewSearchRequest(a.API,
			WithSearchIndex(index),
			WithSearchQueryOptions(query.Since("created_at", since)),
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

		newFeeds = append(newFeeds, feeds...)

		// Stop if we are at the end of the results.
		if int(resp.Hits.Total.Value) < searchSize {
			break
		}
	}

	return newFeeds, nil
}
