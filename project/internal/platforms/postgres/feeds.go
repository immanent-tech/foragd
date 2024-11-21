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

func (c *Client) getFeedByURL(url string) (*models.Feed, error) {
	var feed models.Feed
	if err := c.db.First(&feed, "url = ?", url).Error; err != nil {
		return nil, err
	}

	return &feed, nil
}

func (c *Client) GetUpdatedFeeds(since time.Time) ([]models.Feed, error) {
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
