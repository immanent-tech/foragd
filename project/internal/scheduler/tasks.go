// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"

	"github.com/joshuar/go-feed-me/internal/models"
)

// Task types.
const (
	TypeGetFeedItems = "feed:getItems"
)

var ErrTaskFailed = errors.New("task failed")

type TaskRunner struct {
	cache  Cache
	db     DB
	logger *slog.Logger
}

type getFeedItemsPayload struct {
	Feed models.APIFeed `json:"feed"`
}

func NewGetFeedItemsTask(feed models.APIFeed) (*asynq.Task, error) {
	payload, err := json.Marshal(getFeedItemsPayload{Feed: feed})
	if err != nil {
		return nil, fmt.Errorf("could not marshal task payload: %w", err)
	}

	return asynq.NewTask(TypeGetFeedItems, payload), nil
}

func (r *TaskRunner) HandleGetFeedItemsTask(ctx context.Context, t *asynq.Task) error {
	var payload getFeedItemsPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("could not unmarshal task payload: %w", err)
	}

	// Get the time the feed items were last fetched.
	lastFetched, err := r.db.GetFeedLastFetched(ctx, payload.Feed.ID)
	if err != nil {
		return errors.Join(ErrTaskFailed, err)
	}
	// Get new items since the last fetch.
	items := payload.Feed.GetItemsSince(ctx, lastFetched)
	// Cache the new items.
	if err := r.cache.AddFeedItems(ctx, items...); err != nil {
		return fmt.Errorf("could not cache feed items: %w", err)
	}

	if len(items) > 0 {
		r.logger.Debug("Added new items for feed.",
			slog.String("feed_id", payload.Feed.ID),
			slog.String("title", payload.Feed.Title),
			slog.Int("count", len(items)),
		)
	}

	return nil
}
