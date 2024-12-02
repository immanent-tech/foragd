// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"errors"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/joshuar/go-feed-me/internal/id"
)

// isNewer returns a boolean indicating whether this item has been updated or
// published after the given time.
func (i *Item) isNewer(since time.Time) bool {
	var itemTime time.Time

	if i.UpdatedParsed != nil {
		itemTime = *i.UpdatedParsed
	} else {
		itemTime = *i.PublishedParsed
	}

	return itemTime.After(since)
}

// NewFeedItem creates a new Feed object from the given item details, using the
// given feed ID.
func NewFeedItem(feedID string, details *gofeed.Item) (*Item, error) {
	var err error

	itemID, err := id.NewID(id.Item)
	if err != nil {
		return nil, errors.Join(ErrInvalidID, err)
	}

	return &Item{
			CreatedAt: time.Now().UTC(),
			ID:        itemID,
			FeedID:    feedID,
			Item:      details,
		},
		nil
}
