// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package feeds

import (
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/joshuar/go-feed-me/platform/id"
)

type FeedItem struct {
	*gofeed.Item
	FeedID string `json:"feedID"`
	ItemID string `json:"itemID"`
}

func (i *FeedItem) IsNewer(since time.Time) bool {
	if i.UpdatedParsed != nil {
		return i.UpdatedParsed.After(since)
	}

	return i.PublishedParsed.After(since)
}

func NewFeedItem(feedID string, details *gofeed.Item) FeedItem {
	return FeedItem{
		FeedID: feedID,
		ItemID: newItemID(),
		Item:   details,
	}
}

func newItemID() string {
	feedID, err := id.NewID(id.Item)
	if err != nil {
		return ""
	}

	return feedID
}
