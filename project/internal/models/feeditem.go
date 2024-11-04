// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"time"

	"github.com/mmcdole/gofeed"

	components "github.com/joshuar/go-templ-daisyui"

	"github.com/joshuar/go-feed-me/internal/id"
)

// GetImage extracts the item image and creates an image component that can be
// rendered in a page.
func (i *FeedItem) GetImage() *components.Image {
	if i.Image != nil {
		image := components.NewImage(
			components.WithURL(i.Image.URL),
			components.WithAltText(i.Image.Title),
		)

		return &image
	}

	return nil
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
