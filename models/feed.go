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

	feeds "github.com/immanent-tech/go-syndication"
	"github.com/immanent-tech/go-syndication/types"
	slogctx "github.com/veqryn/slog-context"
)

// ErrNewFeed is returned when there was a problem creating a new Feed.
var ErrNewFeed = errors.New("could not create new feed")

// Feeds is a slice of Feed objects.
type Feeds []*Feed

// GetIDs returns the Feed IDs for the Feeds.
func (f Feeds) GetIDs() []FeedID {
	feedIDs := make([]FeedID, 0, len(f))
	for feed := range slices.Values(f) {
		feedIDs = append(feedIDs, feed.GetID())
	}
	return feedIDs
}

// FindByID will return the feed with the given ID.
func (f Feeds) FindByID(id FeedID) *Feed {
	idx := slices.IndexFunc(f, func(v *Feed) bool { return v.GetID() == id })
	if idx == -1 {
		return nil
	}
	return f[idx]
}

// GetID returns the ID of the Feed.
func (f *Feed) GetID() FeedID {
	return f.FeedID
}

// GetSourceURLs returns all discovered source URLs of the feed (i.e., URLs that point to the feed itself).
func (f *Feed) GetSourceURLs() []URL {
	return f.SourceURLs
}

// GetLink returns the URL of the website that publishes and/or is the owner the feed.
func (f *Feed) GetLink() URL {
	return f.URL
}

// GetTitle returns the feed title.
func (f *Feed) GetTitle() string {
	return f.Title
}

// GetDescription returns the feed description, if any.
func (f *Feed) GetDescription() string {
	return f.Description
}

// GetAuthors returns the feed authors, if any.
func (f *Feed) GetAuthors() []string {
	return f.Authors
}

// GetContributors returns the feed contributors, if any.
func (f *Feed) GetContributors() []string {
	return f.Contributors
}

// GetCategories returns the slice of categories assigned to the feed, if any.
func (f *Feed) GetCategories() []string {
	return f.Categories
}

// GetImage returns an image object that can visually represent the feed.
func (f *Feed) GetImage() *types.ImageInfo {
	return &f.Image
}

// GetItems returns a slice of the currently published items in the feed.
func (f *Feed) GetItems() []types.ItemSource {
	return nil
}

// GetLanguage returns the language tag of the feed, if any.
func (f *Feed) GetLanguage() string {
	return f.Language
}

// GetTimestamp returns a timestamp indicating when the feed was last updated. This will be either, the updated
// timestamp in the feed, or, the published timestamp in the feed, or the last fetched timestamp, whichever is found and
// is a valid value, in that order.
func (f *Feed) GetTimestamp() time.Time {
	if !f.Updated.IsZero() && !f.Updated.Equal(types.UnixEpoch) {
		return f.Updated.UTC()
	}
	if !f.Published.IsZero() && !f.Published.Equal(types.UnixEpoch) {
		return f.Published.UTC()
	}
	return f.LastFetched.UTC()
}

// GetRights returns the rights or copyright of the feed content, if any.
func (f *Feed) GetRights() string {
	return f.Copyright
}

// NewFeedFromURL generates a new Feed object from the given URL. If there is a problem generating the object, a non-nil
// error is returned.
func NewFeedFromURL(ctx context.Context, url string) (*Feed, error) {
	var feed *Feed

	ctx, cancel := context.WithTimeout(ctx, feeds.DefaultRequestTimeout)
	defer cancel()

	results := feeds.NewFeedsFromURLs(ctx, url)
	for result := range slices.Values(results) {
		if result.Err != nil {
			return nil, fmt.Errorf("could not create feed from URL %s: %w", url, result.Err)
		}
		if result.Feed.GetImage() == nil {
			err := feeds.FindFeedImage(ctx, result.Feed)
			if err != nil {
				slogctx.FromCtx(ctx).WarnContext(ctx, "No image for feed.",
					slog.String("feed", result.Feed.GetTitle()),
					slog.String("url", result.Feed.GetSourceURL()),
				)
			}
		}
		feed = NewFeedFromSource(url, result.Feed)
	}

	return feed, nil
}

// NewFeedFromSource converts the raw types.FeedSource into a Feed object.
func NewFeedFromSource(url string, source *feeds.Feed) *Feed {
	feed := &Feed{
		FeedID:       NewID(FeedPFX),
		CreatedAt:    time.Now().UTC(),
		LastFetched:  types.UnixEpoch,
		Published:    source.GetPublishedDate().UTC(),
		Updated:      source.GetUpdatedDate().UTC(),
		Title:        source.GetTitle(),
		Description:  source.GetDescription(),
		SourceType:   FeedSourceType(source.SourceType),
		SourceURLs:   []string{source.GetSourceURL()},
		URL:          source.GetLink(),
		Authors:      source.GetAuthors(),
		Contributors: source.GetContributors(),
		Copyright:    source.GetRights(),
		Language:     source.GetLanguage(),
		Categories:   source.GetCategories(),
	}
	// Add the url used to find the feed to the source URLs if needed.
	if !slices.Contains(feed.SourceURLs, url) {
		feed.SourceURLs = append(feed.SourceURLs, url)
	}
	if source.GetImage() != nil {
		feed.Image = *source.GetImage()
	}

	return feed
}
