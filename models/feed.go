// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

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

// normaliseURL strips protocol handler schemes and cleans the URL
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
