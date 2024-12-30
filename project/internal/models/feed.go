// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/joshuar/go-feed-me/internal/id"
	"github.com/joshuar/go-feed-me/internal/logging"
)

// GetItemsSince retrieves the feed items that are newer than the given time.
func (f *APIFeed) GetItemsSince(ctx context.Context, since time.Time) []Item {
	var items []Item

	details, err := parser.ParseURL(f.URL)
	if err != nil {
		logging.FromContext(ctx).Warn("Problem getting feed details.", slog.Any("error", err))
	}

	for _, i := range details.Items {
		item, err := NewFeedItem(f.ID, i)
		if err != nil {
			logging.FromContext(ctx).Warn("Problem creating new item.", slog.Any("error", err))
			continue
		}

		if !item.isNewer(since) {
			continue
		}

		items = append(items, *item)
	}

	return items
}

func (f *APIFeed) GetUnreadCount(ctx context.Context, cache Cache) int {
	count, err := cache.CountUnread(ctx, f.ID)
	if err != nil {
		logging.FromContext(ctx).Warn("Could not get unread count for feed.",
			slog.String("feed", f.Title),
			slog.String("id", f.ID),
			slog.Any("error", err))

		return 0
	}

	return count
}

func (f *APIFeed) GetTitle() string {
	return f.Title
}

func (f *APIFeed) GetID() string {
	return f.ID
}

func (f *APIFeed) GetImage() *gofeed.Image {
	return f.Image
}

func (f *APIFeed) GetCategories() []string {
	return f.Categories
}

func (f *APIFeed) GetContent() string {
	return f.Description
}

// NewFeedFromURL creates a new feed model from the given URL as its canonical
// data source.
func NewFeedFromURL(url string) (*Feed, error) {
	var err error

	feedID, err := id.NewID(id.Feed)
	if err != nil {
		return nil, errors.Join(ErrInvalidID, err)
	}

	details, err := parser.ParseURL(url)
	if err != nil {
		return nil, errors.Join(ErrParseFeed, err)
	}

	return &Feed{
			CreatedAt: time.Now().UTC(),
			ID:        feedID,
			Feed:      details,
		},
		nil
}

func (f *APIFeed) CacheNewItems(ctx context.Context, cache Cache, db DB) error {
	return nil
}
