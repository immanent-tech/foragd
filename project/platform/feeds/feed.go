// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package feeds

import (
	"fmt"
	"time"

	"github.com/mmcdole/gofeed"
)

type Feed struct {
	*gofeed.Feed
	FeedID string
	URL    string
}

func (f *Feed) GetItemsSince(since time.Time) []FeedItem {
	var items []FeedItem

	for _, i := range f.Items {
		item := NewFeedItem(f.FeedID, i)
		if item.IsNewer(since) {
			items = append(items, item)
		}
	}

	return items
}

func (f *Feed) update() error {
	fp := gofeed.NewParser()

	details, err := fp.ParseURL(f.URL)
	if err != nil {
		return fmt.Errorf("cannot parse feed: %w", err)
	}

	f.Feed = details

	return nil
}

func NewFeed(id, url string) Feed {
	return Feed{
		FeedID: id,
		URL:    url,
	}
}
