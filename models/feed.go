// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	estypes "github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/sortorder"
	"github.com/go-playground/validator/v10"
	feeds "github.com/immanent-tech/go-syndication"
	"github.com/immanent-tech/go-syndication/types"
	slogctx "github.com/veqryn/slog-context"
	"github.com/zeebo/xxh3"

	"github.com/immanent-tech/foragd/client"

	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/reverseproxy"
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

// SetUpdateInterval will set the update interval of the feed. It fetches the feed details from the source and
// determines a reasonable update interval. In the case of errors, a default interval will be set.
func (f *Feed) SetUpdateInterval(ctx context.Context) error {
	// In case of an error, set the default update interval to hourly.
	f.UpdateInterval = int64(time.Hour)

	// Get fresh feed details.
	details, err := feeds.NewFeedFromURL(ctx, f.GetSourceURLs()[0])
	if err != nil {
		slogctx.FromCtx(ctx).
			Warn("Unable to retrieve feed details to determine update interval. Using default update interval.",
				slog.Any("error", err),
			)
		f.UpdateInterval = int64(time.Hour)
		return nil
	}

	// For Atom, assume a default hourly update.
	if details.SourceType == feeds.TypeAtom || details.SourceType == feeds.TypeJSONFeed {
		f.UpdateInterval = int64(time.Hour)
	}

	// For RSS use either the reasonable interval given by the feed or a reasonable default.
	if details.SourceType == feeds.TypeRSS {
		interval := details.GetUpdateInterval()
		switch {
		case interval < time.Minute:
			// Set really short update intervals to every 5 minutes.
			f.UpdateInterval = int64(5 * time.Minute)
		case interval > 24*time.Hour:
			// Set really long update intervals to daily.
			f.UpdateInterval = int64(24 * time.Hour)
		default:
			f.UpdateInterval = int64(interval)
		}
	}

	// Update the feed with the calculated poll interval for reference.
	if err := elastic.UpdateDoc(ctx, schema.FeedsIndexRW(), f.GetID(), map[string]any{
		"update_interval": f.UpdateInterval,
	}); err != nil {
		return fmt.Errorf("set feed update interval: %w", err)
	}

	return nil
}

// FindOrCreateFeed will either generate a new feed or return the existing feed for the given URL. If the feed is new,
// the boolean return value will be true.
func FindOrCreateFeed(ctx context.Context, feedURL string) (*Feed, bool, error) {
	// Fetch from URL as feed.
	newFeed, err := NewFeedFromURL(ctx, feedURL, "", false)
	if err != nil {
		return nil, false, fmt.Errorf("fetch new feed: %w", err)
	}

	// Create terms queries to match the new feed to an existing feed.
	var terms []query.Option
	for url := range slices.Values(newFeed.SourceURLs) {
		terms = append(terms, query.Term("source_urls", url))
		// Also match url with trailing slash.
		if !strings.HasSuffix(url, "/") {
			terms = append(terms, query.Term("source_urls", url+"/"))
		}
	}
	terms = append(terms, query.Term("url", newFeed.URL))
	// Also match url with trailing slash.
	if !strings.HasSuffix(newFeed.URL, "/") {
		terms = append(terms, query.Term("source_urls", newFeed.URL+"/"))
	}
	// Find any existing feed.
	resp, err := elastic.Search[*Feed](ctx,
		schema.FeedsIndexRO(),
		query.Bool(
			query.Filter(
				query.Bool(
					query.Should(terms...),
				),
			),
		),
		elastic.WithSize(1),
	)
	if err != nil {
		return nil, false, fmt.Errorf("search existing feeds: %w", err)
	}
	if len(resp.Results) == 1 {
		// If an existing feed is found, use that feed.
		return resp.Results[0], false, nil
	}
	// Otherwise use the new feed.
	return newFeed, true, nil
}

// NewFeedFromURL generates a new Feed object from the given URL. If there is a problem generating the object, a non-nil
// error is returned.
func NewFeedFromURL(ctx context.Context, rawURL string, id FeedID, validate bool) (*Feed, error) {
	// Parse the raw URL and make any adjustments based on the domain for specific canonical sources.
	feedURL, err := feedURLParser(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	var feed *Feed

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	result, err := feeds.NewFeedFromURL(
		ctx,
		feedURL.String(),
		feeds.PerformValidation(validate),
		feeds.WithClient(client.Load()),
	)
	if err != nil {
		if validateErrs, ok := errors.AsType[validator.ValidationErrors](err); ok && validate {
			slogctx.FromCtx(ctx).Warn("Feed is invalid, continuing without validation",
				slog.String("url", feedURL.String()),
				slog.Any("error", validateErrs),
			)
			// On validation errors, try again without validation.
			return NewFeedFromURL(ctx, feedURL.String(), id, false)
		}
		if parseErr, ok := errors.AsType[feeds.ParseError](
			err,
		); ok &&
			(parseErr.Code == http.StatusForbidden || parseErr.Code == http.StatusTooManyRequests) {
			// If the error is StatusForbidden, or TooManyRequests, try proxying the request.
			if proxied, err := reverseproxy.IsProxiedURL(feedURL.String()); err != nil && !proxied {
				// Generate a proxied URL.
				proxiedURL, err := reverseproxy.GenerateProxyURL(feedURL.String())
				if err != nil {
					return nil, fmt.Errorf("proxy url: %w", err)
				}
				slogctx.FromCtx(ctx).Debug("Proxying feed request.",
					slog.String("url", proxiedURL),
				)
				if feed, err = NewFeedFromURL(ctx, proxiedURL, id, validate); err != nil {
					return nil, err
				}
				// Clean up source URLs: remove proxied URL and re-add original URL as needed.
				feed.SourceURLs = slices.DeleteFunc(feed.SourceURLs, func(sourceURL string) bool {
					proxied, err := reverseproxy.IsProxiedURL(sourceURL)
					switch {
					case proxied:
						return true
					case err != nil:
						slogctx.FromCtx(ctx).Warn("Unable to determine if URL was proxied.",
							slog.String("url", sourceURL),
							slog.Any("error", err))
					}
					return false
				})
				feed.SourceURLs = append(feed.SourceURLs, feedURL.String())
				return feed, err
			}
			// If it has already been proxied and there is a parse error, just return the error.
			return nil, fmt.Errorf("could not create feed from URL %s: %w", feedURL.String(), parseErr)
		}
		// Return the error.
		return nil, fmt.Errorf("could not create feed from URL %s: %w", feedURL.String(), err)
	}

	feed = newSyndicationFeed(ctx, feedURL.String(), id, result)

	// Try to find an image for the feed if it does not supply one.
	if feed.GetImage() == nil {
		// Fetch and extract image from opengraph data (if any).
		img, err := client.ExtractMainImage(ctx, feed.GetLink())
		if err != nil {
			slogctx.FromCtx(ctx).WarnContext(ctx, "No image for feed.",
				slog.String("feed", feed.GetTitle()),
			)
		}
		if img != "" {
			feed.Image = NewRemoteImage(img, feed.GetTitle())
		}
	}

	return feed, nil
}

// newSyndicationFeed converts the raw types.FeedSource into a Feed object.
func newSyndicationFeed(ctx context.Context, url string, id FeedID, source *feeds.Feed) *Feed {
	if id == "" {
		id = "feed_" + strconv.FormatUint(xxh3.Hash([]byte(source.GetLink())), 10)
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
	if pubDate := source.GetPublishedDate(); pubDate != nil {
		feed.Published = pubDate.UTC()
	} else {
		feed.Published = UnixEpoch
	}
	if updatedDate := source.GetUpdatedDate(); updatedDate != nil {
		feed.Updated = new(updatedDate.UTC())
	}

	// Extract Items from source and add to Feed. We do this in parallel as generation of some items may involve network
	// calls to fetch additional information (e.g., images).
	var wg sync.WaitGroup
	itemCh := make(chan Item)
	for i := range slices.Values(source.GetItems()) {
		wg.Go(func() {
			item := NewFeedItem(ctx, &i, feed)
			itemCh <- *item
		})
	}
	go func() {
		defer close(itemCh)
		wg.Wait()
	}()
	for item := range itemCh {
		feed.Items = append(feed.Items, item)
	}

	// Add the url used to find the feed to the source URLs if needed.
	if !slices.Contains(feed.SourceURLs, url) {
		feed.SourceURLs = append(feed.SourceURLs, url)
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

// FeedSorting contains the sort options for sorting item search results.
type FeedSorting struct {
	Updated   string `json:"updated"`
	Published string `json:"published"`
	FeedID    string `json:"feed_id"`
}

// SortCombinationsCaster is required to allow FeedSorting to be used as Elasticsearch sort values.
func (s *FeedSorting) SortCombinationsCaster() *estypes.SortCombinations {
	c := estypes.SortCombinations(s)
	return &c
}

func NewFeedSortOptions(sort *Sort) []estypes.SortCombinationsVariant {
	if sort == nil {
		return []estypes.SortCombinationsVariant{&estypes.SortOptions{Doc_: estypes.NewScoreSort()}}
	}
	var opts []estypes.SortCombinationsVariant
	switch *sort {
	case SortNewestFirst:
		opts = append(opts, &FeedSorting{
			Updated:   "desc",
			Published: "desc",
			FeedID:    "desc",
		})
	case SortOldestFirst:
		opts = append(opts, &FeedSorting{
			Updated:   "asc",
			Published: "asc",
			FeedID:    "asc",
		})
	case SortMostRelevant:
		opts = append(opts, &estypes.SortOptions{
			Score_: &estypes.ScoreSort{
				Order: &sortorder.Desc,
			},
		})
		opts = append(opts,
			&FeedSorting{
				Updated:   "asc",
				Published: "asc",
				FeedID:    "asc",
			},
		)
	default:
		opts = append(opts, &estypes.SortOptions{
			Doc_: estypes.NewScoreSort(),
		})
	}
	return opts
}

func NewFeedSortCombinations(sort *Sort) []estypes.SortCombinations {
	var opts []estypes.SortCombinations
	switch *sort {
	case SortNewestFirst:
		opts = append(opts, &FeedSorting{
			Updated:   "desc",
			Published: "desc",
			FeedID:    "desc",
		})
	case SortOldestFirst:
		opts = append(opts, &FeedSorting{
			Updated:   "asc",
			Published: "asc",
			FeedID:    "asc",
		})
	case SortMostRelevant:
		opts = append(opts, &estypes.SortOptions{
			Score_: &estypes.ScoreSort{
				Order: &sortorder.Desc,
			},
		})
		opts = append(opts,
			&FeedSorting{
				Updated:   "asc",
				Published: "asc",
				FeedID:    "asc",
			},
		)
	default:
		opts = append(opts, &estypes.SortOptions{
			Doc_: estypes.NewScoreSort(),
		})
	}
	return opts
}

// FeedURLParser parses the given URL string into a url.URL object, applying some additional rules for known domains on where
// to find their feeds.
func feedURLParser(ctx context.Context, urlStr string) (*url.URL, error) {
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
				slogctx.FromCtx(ctx).Warn("Could not create subreddit RSS url.",
					slog.Any("err", err),
				)
			}
			slogctx.FromCtx(ctx).Debug("Appended .rss onto end of URL for subreddit.",
				slog.String("original_url", urlStr),
				slog.String("new_url", feedURL.String()),
			)
		}
	case strings.HasSuffix(feedURL.Host, "tumblr.com"):
		switch {
		case !strings.HasPrefix(feedURL.Path, "rss") && !strings.HasPrefix(feedURL.Path, "rss/"):
			// Tumblr blogs usually have their feed at the "/feed" path.
			var err error
			if feedURL.Path, err = url.JoinPath(feedURL.Path, "/rss"); err != nil {
				slogctx.FromCtx(ctx).Warn("Could not create create canonical Tumblr RSS url.",
					slog.Any("err", err),
				)
			}
			slogctx.FromCtx(ctx).Debug("Appended feed onto end of URL for tumblr blog.",
				slog.String("original_url", urlStr),
				slog.String("new_url", feedURL.String()),
			)
		}
	}

	return feedURL, nil
}
