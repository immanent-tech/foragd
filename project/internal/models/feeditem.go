// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"errors"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/joshuar/go-feed-me/internal/id"
)

func (i *Item) isNewer(since time.Time) bool {
	if i.UpdatedParsed != nil {
		return since.After(*i.UpdatedParsed)
	}

	return since.After(*i.PublishedParsed)
}

func NewFeedItem(feedID string, details *gofeed.Item) (*Item, error) {
	var err error

	itemID, err := id.NewID(id.Item)
	if err != nil {
		return nil, errors.Join(ErrInvalidID, err)
	}

	item := &Item{ID: itemID, FeedID: feedID}
	item.Item = details

	return item, nil
}
