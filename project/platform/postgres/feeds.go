// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package postgres

import (
	"fmt"
	"time"

	"github.com/joshuar/go-feed-me/platform/feeds"
	"github.com/joshuar/go-feed-me/platform/id"
)

func (c *Client) getFeedByURL(url string) (*Feed, error) {
	var feed Feed
	if err := c.db.First(&feed, "url = ?", url).Error; err != nil {
		return nil, err
	}

	return &feed, nil
}

func (c *Client) GetUpdatedFeeds(since time.Time) ([]feeds.Feed, error) {
	var results []Feed

	if err := c.db.Where("updated_at > ?", since).Find(&results).Error; err != nil {
		return nil, fmt.Errorf("could not retrieve updated feed list: %w", err)
	}

	updatedFeeds := make([]feeds.Feed, len(results))

	for i, feed := range results {
		updatedFeeds[i] = feeds.NewFeed(feed.ID, feed.URL)
	}

	return updatedFeeds, nil
}

func newFeedRecord(url string) (*Feed, error) {
	feedID, err := id.NewID(id.Feed)
	if err != nil {
		return nil, fmt.Errorf("cannot create feed: %w", err)
	}

	return &Feed{ID: feedID, URL: url}, nil
}
