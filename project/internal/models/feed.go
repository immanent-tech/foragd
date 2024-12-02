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

var ErrParseFeed = errors.New("could not parse feed")

var parser *gofeed.Parser

func init() {
	parser = gofeed.NewParser()
	parser.UserAgent = "Mozilla/5.0 (X11; Linux x86_64; rv:132.0) Gecko/20100101 Firefox/132.0"
}

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
