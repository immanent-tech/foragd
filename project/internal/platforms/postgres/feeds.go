// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package postgres

import (
	"fmt"
	"time"

	"github.com/joshuar/go-feed-me/internal/models"
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
