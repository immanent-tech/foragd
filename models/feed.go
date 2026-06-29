// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	feeds "github.com/immanent-tech/go-syndication"
	"github.com/immanent-tech/go-syndication/types"
	"github.com/zeebo/xxh3"
)

// GetID retrieves (generates) a unique ID for a FeedStatus object.
func (s *FeedStatus) GetID() string {
	return strconv.FormatUint(xxh3.Hash([]byte(s.FeedID+s.Timestamp.String())), 10)
}

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

// ExcludeIDs returns a new slice containing the feeds which DO NOT have an id matching the given IDs.
func (f Feeds) ExcludeIDs(ids ...FeedID) Feeds {
	if len(ids) == 0 {
		return f
	}
	return slices.Collect(
		FilterSlice(f, func(e *Feed) bool {
			return !slices.Contains(ids, e.GetID())
		}),
	)
}

// NewFeed converts a feed source from the go-syndication library into a models.Feed object.
func NewFeed(sourceURL string, id FeedID, source *feeds.Feed) *Feed {
	if id == "" {
		id = "feed_" + strconv.FormatUint(xxh3.Hash([]byte(source.GetSourceURL())), 10)
	}
	feed := &Feed{
		FeedID:       id,
		CreatedAt:    time.Now().UTC(),
		LastFetched:  types.UnixEpoch,
		Title:        source.GetTitle(),
		Description:  new(source.GetDescription()),
		SourceType:   SourceType(source.SourceType),
		SourceURLs:   []string{source.GetSourceURL()},
		URL:          source.GetLink(),
		Authors:      source.GetAuthors(),
		Contributors: source.GetContributors(),
		Copyright:    source.GetRights(),
		Language:     source.GetLanguage(),
		Categories:   source.GetCategories(),
	}

	// Set the published date. If no published date in the source, set it to unix epoch.
	if pubDate := source.GetPublishedDate(); pubDate != nil {
		feed.Published = pubDate.UTC()
	} else {
		feed.Published = UnixEpoch
	}

	// Set the updated date (if found in the source).
	if updatedDate := source.GetUpdatedDate(); updatedDate != nil {
		feed.Updated = new(updatedDate.UTC())
	}

	// Extract the hostname from the link into the domain field.
	link, _ := url.Parse(source.GetLink())
	feed.Domain = link.Hostname()

	// Extract Items from source and add to Feed. We do this in parallel as generation of some items may involve network
	// calls to fetch additional information (e.g., images).
	var wg sync.WaitGroup
	itemCh := make(chan Item)
	for i := range slices.Values(source.GetItems()) {
		wg.Go(func() {
			item := NewFeedItem(&i, feed)
			itemCh <- *item
		})
	}
	go func() {
		defer close(itemCh)
		wg.Wait()
	}()
	for item := range itemCh {
		feed.Items = append(feed.Items, &item)
	}

	// Add the url used to find the feed to the source URLs if needed.
	if !slices.Contains(feed.SourceURLs, sourceURL) {
		feed.SourceURLs = append(feed.SourceURLs, sourceURL)
	}
	// Add any image found.
	if sourceImg := source.GetImage(); sourceImg != nil {
		feed.Image = &RemoteImage{
			URL:   new(sourceImg.GetURL()),
			Title: new(sourceImg.GetTitle()),
		}
	}

	return feed
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
	if f.Description != nil {
		return *f.Description
	}
	return ""
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
func (f *Feed) GetImage() *RemoteImage {
	return f.Image
}

// GetItems returns a slice of the currently published items in the feed.
func (f *Feed) GetItems() Items {
	return f.Items
}

// GetLanguage returns the language tag of the feed, if any.
func (f *Feed) GetLanguage() string {
	if f.Language != nil {
		return *f.Language
	}
	return ""
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
	if f.Copyright != nil {
		return *f.Copyright
	}
	return ""
}

// NormaliseFeedURL strips protocol handler schemes and cleans the URL.
func NormaliseFeedURL(raw string) string {
	// Strip protocol handler prefixes: web+feed://, web+rss://
	for _, prefix := range []string{"web+feed://", "web+rss://", "web+feed:", "web+rss:"} {
		if after, ok := strings.CutPrefix(raw, prefix); ok {
			raw = after
			if !strings.HasPrefix(raw, "http") {
				raw = "https://" + raw
			}
			return raw
		}
	}
	// Decode in case the share target URL-encoded it
	if decoded, err := url.QueryUnescape(raw); err == nil {
		raw = decoded
	}
	return raw
}
