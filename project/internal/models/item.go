// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/joshuar/go-feed-me/internal/id"
)

var ErrGetItem = errors.New("could not retrieve item")

func (i *APIItem) GetTitle() string {
	return i.Title
}

func (i *APIItem) GetID() string {
	return i.ID
}

func (i *APIItem) GetFeedID() string {
	return i.FeedID
}

func (i *APIItem) GetImage() *gofeed.Image {
	return i.Image
}

func (i *APIItem) GetCategories() []string {
	return i.Categories
}

func (i *APIItem) GetContent() string {
	return i.Description
}

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

func GetItem(ctx context.Context, db DB, cache Cache, feedID string, itemID string) (*APIItem, error) {
	// Find a subscription by the provided feed ID.
	found, err := db.IsSubscribed(ctx, feedID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrGetSubscriptions, err)
	}
	// If not subscribed to this feed, return nothing but an error.
	if !found {
		return nil, ErrNotSubscribed
	}

	item, err := cache.GetItem(ctx, feedID, itemID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrGetItem, err)
	}

	return &item, nil
}
