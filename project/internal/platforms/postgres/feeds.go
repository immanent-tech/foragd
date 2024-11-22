// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package postgres

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/server/session"
)

// GetNewItems fetches a feed from the database, then gets any new items from
// the feed source that are newer than the last fetched date.
func (c *Client) GetNewItems(feedID string) ([]models.FeedItem, error) {
	var feed models.APIFeed
	var items []models.FeedItem

	if err := c.db.Transaction(func(tx *gorm.DB) error {
		if err := c.db.Model(&models.Feed{}).First(&feed, "id = ?", feedID).Error; err != nil {
			return fmt.Errorf("could not retrieve feed: %w", err)
		}

		items = feed.GetItemsSince(feed.LastFetched)

		if err := c.db.Model(&models.Feed{}).Where("id = ?", feedID).Update("last_fetched", time.Now()).Error; err != nil {
			return fmt.Errorf("could not update last fetched time: %w", err)
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to get new items for feed: %w", err)
	}

	return items, nil
}

// UpdateLastFetched updates the last fetched time for the given feed.
func (c *Client) UpdateLastFetched(feedID string, lastFetched time.Time) error {
	if err := c.db.Model(&models.Feed{}).Where("id = ?", feedID).Update("last_fetched", lastFetched).Error; err != nil {
		return fmt.Errorf("could not update last fetched time: %w", err)
	}
	return nil
}

func (c *Client) GetNewFeeds(since time.Time) ([]models.Feed, error) {
	var results []models.Feed

	if err := c.db.Where("updated_at > ?", since).Find(&results).Error; err != nil {
		return nil, fmt.Errorf("could not retrieve updated feed list: %w", err)
	}

	return results, nil
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
