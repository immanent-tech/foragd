// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"fmt"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/joshuar/go-feed-me/internal/id"
)

func (f *Feed) GetItemsSince(since time.Time) []FeedItem {
	var items []FeedItem

	for _, i := range f.Items {
		item := NewFeedItem(f.ID, i)
		if item.IsNewer(since) {
			items = append(items, item)
		}
	}

	return items
}

func (f *Feed) Update() error {
	fp := gofeed.NewParser()

	details, err := fp.ParseURL(f.URL)
	if err != nil {
		return fmt.Errorf("cannot parse feed: %w", err)
	}

	f.Feed = details

	return nil
}

func NewFeedFromURL(url string) (*Feed, error) {
	feedID, err := id.NewID(id.Feed)
	if err != nil {
		return nil, fmt.Errorf("cannot create feed: %w", err)
	}

	return &Feed{ID: feedID, URL: url}, nil
}
