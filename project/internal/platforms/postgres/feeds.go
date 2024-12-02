// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/server/session"
)

func (c *Client) GetFeedLastFetched(_ context.Context, feedID string) (time.Time, error) {
	var state models.FeedState

	result := c.db.FirstOrCreate(&state, models.FeedState{ID: feedID})
	if result.Error != nil {
		return time.Now(), fmt.Errorf("could not retrieve last fetched: %w", result.Error)
	}

	return state.LastFetched, nil
}

// UpdateLastFetched updates the last fetched time for the given feed.
func (c *Client) UpdateFeedLastFetched(_ context.Context, feedID string, lastFetched time.Time) error {
	if err := c.db.Model(&models.FeedState{}).Where("id = ?", feedID).Update("last_fetched", lastFetched).Error; err != nil {
		return fmt.Errorf("could not update last fetched time: %w", err)
	}

	return nil
}

// GetSubscribedFeeds returns all feeds that the current user has subscribed to.
func (c *Client) GetSubscribedFeeds(ctx context.Context) ([]models.APIFeed, error) {
	userID, err := session.UserID(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to get subscriptions: %w", err)
	}

	var feeds []models.APIFeed

	results := c.db.Model(&models.Feed{}).
		Joins("JOIN subscriptions ON subscriptions.feed_id = feeds.id AND subscriptions.user_id = ?", userID).
		Find(&feeds)
	if results.Error != nil {
		return nil, fmt.Errorf("unable to get feed subscriptions: %w", err)
	}

	return feeds, nil
}
