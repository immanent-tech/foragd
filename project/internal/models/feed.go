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

var parser = gofeed.NewParser()

var (
	ErrParseFeed       = errors.New("could not parse feed")
	ErrAddFeed         = errors.New("could not add feed")
	ErrNoFeed          = errors.New("feed does not exist")
	ErrNoSubscriptions = errors.New("no user subscriptions")
)

// GetItemsSince retrieves the feed items that are newer than the given time.
func (f *APIFeed) GetItemsSince(ctx context.Context, since time.Time) []Item {
	details, err := parser.ParseURL(f.FeedURL)
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
	return f.FeedURL
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
	if f.UserProperties == nil {
		f.UserProperties = &UserFeedProperties{}
	}

	f.UserProperties.UnreadCount = count
}

func AddFeedByURL(ctx context.Context, api FeedManagementAPI, url URL) (FeedID, error) {
	feed, err := newFeedFromURL(ctx, url)
	if err != nil {
		return "", err
	}
	// Add the feed.
	if err = api.AddFeeds(ctx, *feed); err != nil {
		return "", errors.Join(ErrAddFeed, err)
	}

	return feed.ID, nil
}

func FindOrAddFeed(ctx context.Context, api FeedManagementAPI, url URL) (FeedID, error) {
	var feedID FeedID
	// Find any existing feed with the given subscription URL.
	feed, err := api.GetFeedByURL(ctx, url)
	if err != nil && !errors.Is(err, ErrNoFeed) {
		return "", errors.Join(ErrBackend, err)
	}
	// If there is no existing feed, create a new feed.
	if errors.Is(err, ErrNoFeed) {
		feedID, err = AddFeedByURL(ctx, api, url)
		if err != nil {
			return "", errors.Join(ErrBackend, err)
		}
	} else {
		feedID = feed.ID
	}

	return feedID, nil
}

// newFeedFromURL creates a new feed model from the given URL as its canonical
// data source.
func newFeedFromURL(ctx context.Context, url string) (*Feed, error) {
	var err error

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	feedID, err := id.NewID(id.Feed)
	if err != nil {
		return nil, fmt.Errorf("%w (%s)", err, url)
	}

	details, err := parser.ParseURLWithContext(url, ctx)
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
