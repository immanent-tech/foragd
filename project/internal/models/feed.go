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

func (f *Feed) getDetails() (FeedDetails, error) {
	fp := gofeed.NewParser()

	details, err := fp.ParseURL(f.URL)
	if err != nil {
		return FeedDetails{}, fmt.Errorf("cannot parse feed: %w", err)
	}

	return FeedDetails(*details), nil
}

func (f *Feed) GetItemsSince(since time.Time) []FeedItem {
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

// Update parses the feed using its URL, updating its details and items.
func (f *Feed) Update() error {
	details, err := f.getDetails()
	if err != nil {
		return fmt.Errorf("cannot update feed: %w", err)
	}

	f.Description = details.Description
	f.Title = details.Title

	if details.Image != nil {
		f.ImageURL = details.Image.URL
		f.ImageTitle = details.Image.Title
	}

	return nil
}

// PopulateFromURL populates the feed details using the given URL as its
// canonical data source.
func (f *Feed) PopulateFromURL(url string) error {
	var err error

	f.URL = url

	f.ID, err = id.NewID(id.Feed)
	if err != nil {
		return fmt.Errorf("cannot create feed id: %w", err)
	}

	if err := f.Update(); err != nil {
		return fmt.Errorf("cannot get feed details: %w", err)
	}

	return nil
}
