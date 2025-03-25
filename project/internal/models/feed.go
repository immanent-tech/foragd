// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
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

type FeedInterface interface {
	GetID() FeedID
	GetTitle() string
	GetDescription() string
	GetAuthors() []*gofeed.Person
	GetCategories() []Category
	GetImage() *gofeed.Image
}

type Feeds []*APIFeed

// GetIDs returns the Feed IDs for the Feeds.
func (f Feeds) GetIDs() []FeedID {
	feedIDs := make([]FeedID, len(f))
	for feed := range slices.Values(f) {
		feedIDs = append(feedIDs, feed.GetID())
	}
	return feedIDs
}

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

// GetTitle retrieves either a user-set nickname for the feed or the feed's
// original title.
func (f *APIFeed) GetTitle() string {
	return safePrinter.Sanitize(f.Title)
}

// GetTitle retrieves either a user-set nickname for the feed or the feed's
// original title.
func (f *APIFeed) GetDescription() string {
	return safePrinter.Sanitize(f.Description)
}

// GetTitle retrieves either a user-set nickname for the feed or the feed's
// original title.
func (f *APIFeed) GetAuthors() []*gofeed.Person {
	return f.Authors
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
func (f *APIFeed) GetCategories() []Category {
	return CleanCategories(f.Categories...)
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

func (m *FeedMetadata) GetImage() *gofeed.Image {
	return m.Image
}
