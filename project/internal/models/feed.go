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

var ErrParseFeedFailed = errors.New("could not parse feed")

// ErrNoSubscriptions indicates the user has no subscriptions. In this
// case, a page with a prompt to add subscriptions should be displayed.
var ErrNoSubscriptions = errors.New("no user subscriptions")

// GetItemsSince retrieves the feed items that are newer than the given time.
func (f *APIFeed) GetItemsSince(ctx context.Context, since time.Time) []Item {
	details, err := parser.ParseURL(f.URL)
	if err != nil {
		logging.FromContext(ctx).Warn("Problem getting feed details.", slog.Any("error", err))
	}

	items := make([]Item, 0, len(details.Items))

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

	if len(items) > 0 {
		logging.FromContext(ctx).Debug("Found new items.",
			slog.String("feed", f.GetTitle()),
			slog.String("id", f.GetID()),
			slog.Int("count", len(items)))
	}

	return items
}

func (f *APIFeed) GetTitle() string {
	return safePrinter.Sanitize(f.Title)
}

func (f *APIFeed) GetID() string {
	return f.ID
}

func (f *APIFeed) GetLink() string {
	return f.URL
}

func (f *APIFeed) GetImage() *gofeed.Image {
	return f.Image
}

func (f *APIFeed) GetCategories() []Category {
	if len(f.Categories) == 0 {
		return nil
	}

	cleaned := make([]string, 0, len(f.Categories))

	for _, category := range f.Categories {
		cleaned = append(cleaned, CleanCategory(category))
	}

	return cleaned
}

func (f *APIFeed) GetContent() string {
	return safePrinter.Sanitize(f.Description)
}

func (f *APIFeed) GetTimestamp() time.Time {
	if f.UpdatedAt != nil {
		return *f.UpdatedAt
	}

	return time.Time{}
}

func (f *APIFeed) GetUserUnreadCount() int {
	if userProps := f.UserProperties; userProps != nil {
		return userProps.UnreadCount
	}

	return 0
}

func (f *APIFeed) SetUserUnreadCount(count int) {
	if userProps := f.UserProperties; userProps != nil {
		f.UserProperties = &UserFeedProperties{}
	}

	f.UserProperties.UnreadCount = count
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
