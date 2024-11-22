// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"

	"github.com/joshuar/go-feed-me/internal/models"
)

// Task types.
const (
	TypeGetFeedItems = "feed:getItems"
)

type TaskRunner struct {
	db     dbAPI
	cache  cacheAPI
	logger *slog.Logger
}

type getFeedItemsPayload struct {
	FeedID string `json:"feed_id"`
}

func NewGetFeedItemsTask(feedID string) (*asynq.Task, error) {
	payload, err := json.Marshal(getFeedItemsPayload{FeedID: feedID})
	if err != nil {
		return nil, fmt.Errorf("could not marshal task payload: %w", err)
	}

	return asynq.NewTask(TypeGetFeedItems, payload), nil
}

func (r *TaskRunner) HandleGetFeedItemsTask(_ context.Context, t *asynq.Task) error {
	var payload getFeedItemsPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("could not unmarshal task payload: %w", err)
	}

	itemCh := make(chan models.FeedItem)
	defer close(itemCh)

	go r.cache.CacheFeedItems(itemCh)

	r.logger.Debug("Getting new items for feed.",
		slog.String("feed_id", payload.FeedID))

	items, err := r.db.GetNewItems(payload.FeedID)
	if err != nil {
		return fmt.Errorf("could not get new items for feed: %w", err)
	}

	for _, item := range items {
		itemCh <- item
	}

	return nil
}
