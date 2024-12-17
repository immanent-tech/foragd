// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/a-h/templ"
	"github.com/mmcdole/gofeed"

	components "github.com/joshuar/go-templ-daisyui"

	"github.com/joshuar/go-feed-me/internal/id"
)

var ErrGetItem = errors.New("could not retrieve item")

// AsCardSummary displays a summary of a item in a DaisyUI card component.
// Useful for displaying as a list/grouping of items.
func (i *APIItem) AsCardSummary() components.Card {
	card := components.NewCard(
		components.WithCardLayout(components.CardLayoutSide),
		components.WithTitle(i.Title),
		components.WithCardShadow(components.XL),
		components.WithID[components.Card](i.ID),
		components.WithAttributes[components.Card](templ.Attributes{
			"hx-target": "#content",
			"hx-get":    "/home/" + i.FeedID + "-" + i.ID,
		}),
	)

	if i.Image != nil {
		image := components.NewImage(
			components.WithURL(i.Image.URL),
			components.WithAltText(i.Image.Title),
			components.WithClasses[components.Image]("max-h-full"),
		)
		card.Image = &image
	}

	card.Badges = i.CategoryBadges()

	return card
}

func (i *APIItem) AsArticle() components.Article {
	return components.NewArticle(i.Title, i.Description)
}

func (i *APIItem) CategoryBadges() []components.Badge {
	var badges []components.Badge

	if len(i.Categories) > 0 {
		for _, c := range i.Categories {
			badges = append(badges, components.NewBadge(c))
		}
	}

	return badges
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
