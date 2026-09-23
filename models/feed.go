// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"fmt"
	"html"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/immanent-tech/go-base/validation"
	feeds "github.com/immanent-tech/go-syndication"
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

// GetCategories returns all categories across all the subscriptions. Duplicates are removed.
func (f Feeds) GetCategories() Categories {
	categories := make(Categories, 0)
	for feed := range slices.Values(f) {
		categories = append(categories, feed.GetCategories()...)
	}
	return slices.Compact(categories)
}

// NewFeed converts a feed source from the go-syndication library into a models.Feed object.
func NewFeed(sourceURL string, id FeedID, source *feeds.Feed) *Feed {
	if id == "" {
		id = "feed_" + strconv.FormatUint(xxh3.Hash([]byte(source.GetSourceURL())), 10)
	}
	feed := &Feed{
		FeedID:       id,
		CreatedAt:    time.Now().UTC(),
		LastFetched:  UnixEpoch,
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
	if link, err := url.Parse(source.GetLink()); err == nil {
		feed.Domain = link.Hostname()
	}

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
			URL:   sourceImg.GetURL(),
			Title: new(sourceImg.GetTitle()),
		}
	}

	return feed
}

func (f Feed) Validate() error {
	if f.FetchOptions != nil {
		// If the feed has fetch_options, validate the options are appropriate for the fetch type.
		switch f.FetchMethod {
		case FeedFetchMethodDirect, FeedFetchMethodProxied:
			if _, err := f.FetchOptions.AsFetchDirectOptions(); err != nil {
				return fmt.Errorf("validate feed: invalid fetch options: %w", err)
			}
		case FeedFetchMethodZyteArticles, FeedFetchMethodZyteProducts:
			if _, err := f.FetchOptions.AsFetchZyteOptions(); err != nil {
				return fmt.Errorf("validate feed: invalid fetch options: %w", err)
			}
		}
	}
	if err := validation.Validate.Struct(f); err != nil {
		return fmt.Errorf("validate feed: %w", err)
	}
	return nil
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
	if f.Customisation != nil && f.Customisation.Title != nil {
		return *f.Customisation.Title
	}
	return html.UnescapeString(f.Title)
}

// GetDescription returns the feed description, if any.
func (f *Feed) GetDescription() string {
	if f.Customisation != nil && f.Customisation.Description != nil {
		return *f.Customisation.Description
	}
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
	if f.Updated != nil && (!f.Updated.IsZero() && !f.Updated.Equal(UnixEpoch)) {
		return f.Updated.UTC()
	}
	if !f.Published.IsZero() && !f.Published.Equal(UnixEpoch) {
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

// GetUpdateInterval returns the update interval of the feed. This will be a minimum of every 5 minutes, even if the
// actual update interval is less than 5 minutes.
func (f *Feed) GetUpdateInterval() time.Duration {
	interval := time.Duration(f.UpdateInterval)
	if interval < 5*time.Minute {
		return 5 * time.Minute
	}
	return interval
}

// SetDirectFetchOptions will set the given options for direct fetching on the feed.
func (f *Feed) SetDirectFetchOptions(options ...DirectFetchOption) {
	for option := range slices.Values(options) {
		option(f)
	}
}

// DirectFetchOption is a functional option for applying direct fetch options to a feed.
type DirectFetchOption func(*Feed)

// SetDirectFetchWithItemSummaries option will ensure the feed fetches item summaries separately instead of relying on
// the summaries within the feed.
func SetDirectFetchWithItemSummaries(value bool) DirectFetchOption {
	var fetchOptions FetchDirectOptions
	return func(f *Feed) {
		// Ignore if feed is not configured for direct fetching.
		if f.FetchMethod != FeedFetchMethodDirect {
			return
		}
		// Extract current fetch options.
		if f.FetchOptions == nil {
			fetchOptions = FetchDirectOptions{}
		} else {
			var err error
			fetchOptions, err = f.FetchOptions.AsFetchDirectOptions()
			if err != nil {
				return
			}
		}
		fetchOptions.FetchItemSummaries = value
		f.FetchOptions.FromFetchDirectOptions(fetchOptions)
	}
}

// NormalizeFeedURL parses the given URL string into a [*url.URL] object, applying some additional rules for known
// domains on where to find their feeds.
func NormalizeFeedURL(urlStr string) (*url.URL, error) {
	// Strip protocol handler prefixes: web+feed://, web+rss://.
	for _, prefix := range []string{"web+feed://", "web+rss://", "web+feed:", "web+rss:"} {
		if after, ok := strings.CutPrefix(urlStr, prefix); ok {
			urlStr = after
		}
	}
	// Decode in case it is URL-encoded.
	if decoded, err := url.QueryUnescape(urlStr); err == nil {
		urlStr = decoded
	}
	// Parse the URL.
	feedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	// For some popular sites that have an API or special URL for feeds, handle those.
	switch {
	case strings.Contains(feedURL.Host, "reddit.com"):
		switch {
		case !strings.HasSuffix(feedURL.Path, ".rss") && !strings.HasPrefix(feedURL.Path, ".rss/"):
			// Reddit can usually support a feed by appending `.rss` to the end of the subreddit URL.
			var err error
			if feedURL.Path, err = url.JoinPath(feedURL.Path, "/.rss"); err != nil {
				return nil, fmt.Errorf("generate RSS feed for reddit.com URL: %w", err)
			}
		}
	case strings.HasSuffix(feedURL.Host, "tumblr.com"):
		// Tumblr's canonical feed path is /rss.
		if feedURL.Path != "/rss" {
			feedURL.Path = "/rss"
		}
	case strings.HasSuffix(feedURL.Host, "substack.com"):
		// Substack's canonical feed path is /feed.
		if feedURL.Path != "/feed" {
			feedURL.Path = "/feed"
		}
	case strings.Contains(feedURL.Host, "medium.com") && !strings.Contains(feedURL.Path, "feed"):
		// https://help.medium.com/hc/en-us/articles/214874118-Using-RSS-feeds-of-profiles-publications-and-topics.
		var err error
		if feedURL.Path, err = url.JoinPath("/feed", feedURL.Path); err != nil {
			return nil, fmt.Errorf("generate RSS feed for medium.com URL: %w", err)
		}
	}

	return feedURL, nil
}
