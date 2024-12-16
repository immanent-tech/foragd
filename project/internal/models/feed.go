// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/a-h/templ"

	components "github.com/joshuar/go-templ-daisyui"

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

// AsCardSummary displays a summary of a feed item in a DaisyUI card component.
// Useful for displaying as a list/grouping of feeds.
func (f *APIFeed) AsCardSummary() components.Card {
	card := components.NewCard(
		components.WithCardLayout(components.CardLayoutSide),
		components.WithTitle(f.Title),
		components.WithCardShadow(components.SM),
		components.WithID[components.Card](f.ID),
		components.WithAttributes[components.Card](templ.Attributes{
			"hx-target": "#content",
			"hx-post":   "/home/items",
		}),
		components.WithBody(templ.Raw(f.Description)),
	)

	if f.Image != nil {
		image := components.NewImage(
			components.WithURL(f.Image.URL),
		)

		if f.Image.Title != "" {
			image.Alt = f.Image.Title
		}

		card.Image = &image
	}

	if len(f.Categories) > 0 {
		var categories []components.Badge
		for _, c := range f.Categories {
			categories = append(categories, components.NewBadge(c))
		}

		card.Badges = categories
	}

	return card
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
