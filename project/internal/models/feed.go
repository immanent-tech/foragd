// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/joshuar/go-feed-me/internal/id"
	"github.com/joshuar/go-feed-me/internal/logging"
)

// Ensures we statisfy the ServerInterface interface.
// var _ panes.Feed = (*APIFeed)(nil)

var Parser = gofeed.NewParser()

var (
	ErrParseFeed       = errors.New("could not parse feed")
	ErrAddFeed         = errors.New("could not add feed")
	ErrNoFeed          = errors.New("feed does not exist")
	ErrNoSubscriptions = errors.New("no user subscriptions")
)

// GetItemsSince retrieves the feed items that are newer than the given time.
func (f *APIFeed) GetItemsSince(ctx context.Context, since time.Time) []Item {
	details, err := Parser.ParseURL(f.FeedURL)
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

		if !item.IsNewer(since) {
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
	if userProps := f.UserProperties; userProps != nil {
		if userProps.Nickname != nil {
			return safePrinter.Sanitize(*userProps.Nickname)
		}
	}

	return safePrinter.Sanitize(f.Title)
}

func (f *APIFeed) GetID() string {
	return f.ID
}

func (f *APIFeed) GetLink() string {
	return f.FeedURL
}

func (f *APIFeed) GetImage() *gofeed.Image {
	return f.Image
}

// GetCategories retrieves the list of user-defined and feed-defined categories
// for the Feed.
//
//nolint:prealloc // it would be complicated to pre-allocate the slice.
func (f *APIFeed) GetCategories() []Category {
	var categories []Category

	// Add any user-defined categories.
	if userProps := f.UserProperties; userProps != nil {
		if userProps.Categories != nil {
			for _, category := range *userProps.Categories {
				categories = append(categories, CleanCategory(category))
			}
		}
	}
	// Add any feed categories.
	for _, category := range f.Categories {
		categories = append(categories, CleanCategory(category))
	}

	return categories
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
		if userProps.UnreadCount != nil {
			return *userProps.UnreadCount
		}
	}

	return 0
}

// SetUserName sets the user-defined name in the user properties of the Feed.
func (f *APIFeed) SetUserName(name string) {
	if f.UserProperties == nil {
		f.UserProperties = &UserFeedProperties{}
	}

	f.UserProperties.Nickname = &name
}

// SetUserCategories sets any user-defined categories in the user properties of
// the Feed.
func (f *APIFeed) SetUserCategories(categories []Category) {
	if f.UserProperties == nil {
		f.UserProperties = &UserFeedProperties{}
	}

	f.UserProperties.Categories = &categories
}

// SetUserUnreadCount sets the unread count in the user properties of the Feed.
func (f *APIFeed) SetUserUnreadCount(count int) {
	if f.UserProperties == nil {
		f.UserProperties = &UserFeedProperties{}
	}

	f.UserProperties.UnreadCount = &count
}

// NewFeedFromURL creates a new feed model from the given URL as its canonical
// data source.
func NewFeedFromURL(ctx context.Context, url string) (*Feed, error) {
	var err error

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	feedID, err := id.NewID(id.Feed)
	if err != nil {
		return nil, fmt.Errorf("%w (%s)", err, url)
	}

	details, err := Parser.ParseURLWithContext(url, ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w (%s)", ErrParseFeed, err, url)
	}

	return &Feed{
			CreatedAt: time.Now().UTC(),
			ID:        feedID,
			Feed:      details,
		},
		nil
}
