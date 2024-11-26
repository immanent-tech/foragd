// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/joshuar/go-feed-me/internal/id"
)

type FeedDetails gofeed.Feed

func (f *APIFeed) getDetails() (FeedDetails, error) {
	fp := gofeed.NewParser()

	details, err := fp.ParseURL(f.URL)
	if err != nil {
		return FeedDetails{}, fmt.Errorf("cannot parse feed: %w", err)
	}

	return FeedDetails(*details), nil
}

func (f *APIFeed) GetItemsSince(since time.Time) []FeedItem {
	var items []FeedItem

	details, err := f.getDetails()
	if err != nil {
		slog.Warn("Problem getting feed details.", slog.Any("error", err))
	}

	for _, i := range details.Items {
		item := NewFeedItem(f.ID, i)
		if item.IsNewer(since) {
			items = append(items, item)
		}
	}

	return items
}

// NewFeedFromURL creates a new feed model from the given URL as its canonical
// data source.
func NewFeedFromURL(url string) (*Feed, error) {
	var err error

	fp := gofeed.NewParser()

	details, err := fp.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("cannot parse feed: %w", err)
	}

	feedID, err := id.NewID(id.Feed)
	if err != nil {
		return nil, fmt.Errorf("cannot create feed id: %w", err)
	}

	feed := &Feed{
		URL:         url,
		Title:       details.Title,
		Description: details.Description,
	}
	feed.ID = feedID

	if len(details.Categories) > 0 {
		for _, c := range details.Categories {
			feed.Categories = append(feed.Categories, &c)
		}
	}

	if details.Image != nil {
		feed.ImageURL = &details.Image.URL
		feed.ImageTitle = &details.Image.Title
	}

	return feed, nil
}
