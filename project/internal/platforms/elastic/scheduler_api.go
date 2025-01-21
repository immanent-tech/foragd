// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"time"

	"github.com/joshuar/go-feed-me/internal/models"
)

func (c *Client) GetFeedJobState(ctx context.Context, feedID models.FeedID) (models.FeedJobState, error) {
	index := JobsIndexFromCtx(ctx)
	if index == "" {
		return models.FeedJobState{ID: feedID, LastFetched: time.Time{}}, errors.Join(ErrGetFailed, ErrNoIndexInCtx)
	}

	resp, err := c.NewGetRequest(index, feedID).Do(ctx)
	if err != nil {
		return models.FeedJobState{ID: feedID, LastFetched: time.Time{}}, errors.Join(ErrGetFailed, err)
	}

	// Stop if there are no hits
	if !resp.Found {
		return models.FeedJobState{ID: feedID, LastFetched: time.Time{}}, errors.Join(ErrGetFailed, models.ErrNoJob)
	}

	// Loop through this set of results.
	state, err := ExtractSource[models.FeedJobState](resp.Source_)
	if err != nil {
		return models.FeedJobState{ID: feedID, LastFetched: time.Time{}}, errors.Join(ErrGetFailed, err)
	}

	return state, nil
}

// UpdateFeedJobState will update the job for a feed in the scheduler jobs
// index. Specifically, it will update the last_fetched value indicating when
// the feed last fetched its items.
func (c *Client) UpdateFeedJobState(ctx context.Context, feedID models.FeedID, lastFetched time.Time) error {
	index := JobsIndexFromCtx(ctx)
	if index == "" {
		return errors.Join(ErrGetFailed, ErrNoIndexInCtx)
	}

	if _, err := c.NewDocUpdateRequest(index, feedID,
		WithDocUpdate(&models.FeedJobState{
			ID:          feedID,
			LastFetched: lastFetched,
		}, true),
	).Do(ctx); err != nil {
		return errors.Join(ErrUpdateFailed, err)
	}

	return nil
}
