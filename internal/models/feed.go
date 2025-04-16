// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/joshuar/go-feed-me/internal/id"
	"github.com/joshuar/go-feed-me/pkg/feeds"
	"github.com/joshuar/go-feed-me/pkg/feeds/types"
)

var (
	_ types.ObjectCommon = (*Feed)(nil)
	_ types.Source       = (*Feed)(nil)
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

func (f Feeds) FindByID(id FeedID) *Feed {
	idx := slices.IndexFunc(f, func(v *Feed) bool { return v.GetID() == id })
	if idx == -1 {
		return nil
	}
	return f[idx]
}

func (f Feeds) FindByURL(url string) *Feed {
	idx := slices.IndexFunc(f, func(v *Feed) bool { return v.GetSourceURL() == url })
	if idx == -1 {
		return nil
	}
	return f[idx]
}

func (f *Feed) String() string {
	if f.GetTitle() != "" {
		return fmt.Sprintf("%s (%s)", f.GetTitle(), f.GetSourceURL())
	}
	return f.GetSourceURL()
}

func (f *Feed) GetID() FeedID {
	return f.FeedID
}

func (f *Feed) GetSourceURL() URL {
	return f.SourceURL
}

func (f *Feed) SetSourceURL(url string) {
	f.SourceURL = url
}

func (f *Feed) GetLink() URL {
	return f.URL
}

func (f *Feed) GetTitle() string {
	return f.Title
}

func (f *Feed) GetDescription() string {
	return f.Description
}

func (f *Feed) GetAuthors() []string {
	return f.Authors
}

func (f *Feed) GetContributors() []string {
	return f.Contributors
}

func (f *Feed) GetCategories() []string {
	return f.Categories
}

func (f *Feed) GetImage() *types.Image {
	return f.Image
}

func (f *Feed) GetItems() []types.ItemSource {
	return nil
}

func (f *Feed) GetLanguage() string {
	return f.Language
}

func (f *Feed) GetPublishedDate() time.Time {
	return f.Published
}

func (f *Feed) GetUpdatedDate() time.Time {
	return f.Updated
}

func (f *Feed) GetRights() string {
	return f.Copyright
}

// NewFeedFromURL generates a new Feed object from the given URL. If there is a problem generating the object, a non-nil
// error is returned.
func NewFeedFromURL(ctx context.Context, url string) (*Feed, error) {
	var feed *Feed

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	results := feeds.NewFeedsFromURLs(ctx, url)
	for result := range slices.Values(results) {
		if result.Err != nil {
			return nil, NewMessage("Could not create feed from URL "+url, MessageStatusError, WithError(result.Err))
		}
		feed = NewFeedFromSource(result.Feed.FeedSource, string(result.Feed.SourceType))
	}

	return feed, nil
}

// NewFeedFromSource converts the raw types.FeedSource into a Feed object.
func NewFeedFromSource[T types.FeedSource](source T, sourceType string) *Feed {
	feed := &Feed{
		FeedID:       id.NewID(id.Feed),
		CreatedAt:    time.Now().UTC(),
		Published:    source.GetPublishedDate(),
		Updated:      source.GetUpdatedDate(),
		Title:        source.GetTitle(),
		Description:  source.GetDescription(),
		SourceType:   FeedSourceType(sourceType),
		SourceURL:    source.GetSourceURL(),
		URL:          source.GetLink(),
		Authors:      source.GetAuthors(),
		Contributors: source.GetContributors(),
		Copyright:    source.GetRights(),
		Language:     source.GetLanguage(),
		Categories:   source.GetCategories(),
		Image:        source.GetImage(),
	}

	return feed
}
