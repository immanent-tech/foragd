// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	estypes "github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/calendarinterval"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/sortorder"
	"github.com/go-playground/validator/v10"
	feeds "github.com/immanent-tech/go-syndication"
	"github.com/immanent-tech/go-syndication/types"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/aggregations"
	"github.com/immanent-tech/foragd/providers/elastic/query"
)

// GetFeedByID fetches the given Feed by its id.
func GetFeedByID(ctx context.Context, id FeedID) (*Feed, error) {
	feed, err := elastic.GetDoc[FeedID, *Feed](ctx, schema.FeedsIndexRO, id)
	if err != nil {
		return nil, fmt.Errorf("get feed by id: %w", err)
	}
	return feed, nil
}

// UpdateFeed applies the given updates to a Feed.
func UpdateFeed(ctx context.Context, id FeedID, updates map[string]any) error {
	if err := elastic.UpdateDoc(ctx, schema.SchedulerIndexRW, id, updates); err != nil {
		return fmt.Errorf("update feed: %w", err)
	}
	return nil
}

// GetID retrieves (generates) a unique ID for a FeedStatus object.
func (s *FeedStatus) GetID() string {
	return strconv.FormatUint(xxhash.Sum64String(s.FeedID+s.Timestamp.String()), 10)
}

// AddFeedStatus adds a FeedStatus document to the index.
func AddFeedStatus(ctx context.Context, status *FeedStatus) error {
	if err := elastic.CreateDoc(ctx, schema.FeedStatusIndex, status.GetID(), status); err != nil {
		return ElasticsearchToAPIError(err)
	}
	return nil
}

func getFeedUnreadCounts(
	ctx context.Context,
	subscriptions Subscriptions,
) (map[FeedID]int64, error) {
	// Retrieve user object.
	user := UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user data: %w", ErrCtxValueNotFound)
	}

	// Generate unread count query.
	subscriptionQueries := make([]query.Option, 0, len(subscriptions))
	for subscription := range slices.Values(subscriptions) {
		if subscription.GetSubscriptionType() != SubscriptionTypeFeed &&
			subscription.GetSubscriptionType() != SubscriptionTypeEmail {
			continue
		}
		subscriptionQueries = append(subscriptionQueries, queryUnreadItems(user, subscription))
	}
	// Build query.
	query := query.Bool(
		query.Filter(
			query.Bool(
				query.Should(subscriptionQueries...),
			),
		),
	)
	// Build aggregations.
	termsField := "feed_id"
	termsCount := len(subscriptions)
	aggs := aggregations.Aggs{
		"UnreadCounts": estypes.Aggregations{
			Terms: &estypes.TermsAggregation{
				Field: &termsField,
				Size:  &termsCount,
			},
		},
	}
	// Perform aggregation.
	results, err := ItemsAggregation(ctx, query, 0, aggs)
	if err != nil {
		return nil, fmt.Errorf("unable to get subscription unread counts: %w", err)
	}

	unreadCounts, ok := results.Aggregations["UnreadCounts"].(*estypes.StringTermsAggregate)
	if !ok {
		return nil, fmt.Errorf(
			"unable to get feed stats: UnreadCounts aggregations invalid: %w",
			ErrInvalidAPIResult,
		)
	}
	unreadCountsBuckets, ok := unreadCounts.Buckets.([]estypes.StringTermsBucket)
	if !ok {
		return nil, fmt.Errorf(
			"unable to get feed stats: UnreadCounts aggregations invalid: %w",
			ErrInvalidAPIResult,
		)
	}

	stats := make(map[SubscriptionID]int64)

	// Loop through the aggregation results and extract the unread count for each feed.
	for feed := range slices.Values(unreadCountsBuckets) {
		var feedID FeedID
		if feedID, ok = feed.Key.(string); ok {
			stats[feedID] = feed.DocCount
		}
	}
	return stats, nil
}

func getFeedLastUpdates(ctx context.Context, ids ...FeedID) (map[FeedID]time.Time, error) {
	sort := SortNewestFirst
	items, _, err := elastic.Search[*Item](
		ctx,
		schema.ItemsIndexRO,
		query.Terms("feed_id", ids...),
		len(ids),
		elastic.WithCollapseField("feed_id"),
		elastic.WithSortOptions[*search.Search, elastic.SearchRequest](newItemSortOptions(&sort)...),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to get feed last updates: %w", err)
	}

	updates := make(map[FeedID]time.Time)

	for item := range slices.Values(items) {
		updates[item.GetFeedID()] = item.GetTimestamp()
	}

	return updates, nil
}

// GetFeedSubscriptionStats fetches the stats for FeedSubscriptions and returns a map of the SubscriptionID to
// SubscriptionStats that can be used to lookup the stats pertaining to a particular subscription.
func getFeedAverageDailyUpdates(ctx context.Context, ids ...FeedID) (map[FeedID]float64, error) {
	// Build query.
	query := query.Bool(
		query.WithBoolQueryName("feed_stats_query"),
		query.Filter(
			// Must match any of the given feed IDs.
			query.Terms("feed_id", ids...),
			// Must be published within last month.
			query.Since("@timestamp", time.Now().UTC().Add(-24*30*time.Hour)),
		),
	)
	// Build aggregations.
	termsField := "feed_id"
	termsCount := len(ids)
	dateHistoField := "@timestamp"
	dateFormat := "yyyy-MM-dd"
	aggs := aggregations.Aggs{
		"feed": estypes.Aggregations{
			Terms: &estypes.TermsAggregation{
				Field: &termsField,
				Size:  &termsCount,
			},
			Aggregations: map[string]estypes.Aggregations{
				"updates_per_day": {
					DateHistogram: &estypes.DateHistogramAggregation{
						Field:            &dateHistoField,
						CalendarInterval: &calendarinterval.Day,
						Format:           &dateFormat,
					},
				},
				"avg_daily_updates": {
					AvgBucket: &estypes.AverageBucketAggregation{
						BucketsPath: "updates_per_day._count",
					},
				},
			},
		},
	}

	results, err := ItemsAggregation(ctx, query, len(ids), aggs)
	if err != nil {
		return nil, fmt.Errorf("unable to get feed stats: Feed aggregation invalid: %w", ErrInvalidAPIResult)
	}
	feedStats, ok := results.Aggregations["feed"].(*estypes.StringTermsAggregate)
	if !ok {
		return nil, fmt.Errorf("unable to get feed stats: Feed aggregation invalid: %w", ErrInvalidAPIResult)
	}
	feedStatsBuckets, ok := feedStats.Buckets.([]estypes.StringTermsBucket)
	if !ok {
		return nil, fmt.Errorf("unable to get feed stats: Feed aggregation invalid: %w", ErrInvalidAPIResult)
	}

	stats := make(map[FeedID]float64)

	// Loop through the aggregation results and extract the daily updates metric for each feed.
	for feed := range slices.Values(feedStatsBuckets) {
		var feedID FeedID
		feedID, ok = feed.Key.(string)
		if !ok {
			slogctx.FromCtx(ctx).Debug("Unable to extract feed ID for aggregation", slog.Any("feed_id", feed.Key))
			continue
		}
		var updatesResult *estypes.SimpleValueAggregate
		updatesResult, ok = feed.Aggregations["avg_daily_updates"].(*estypes.SimpleValueAggregate)
		if !ok {
			slogctx.FromCtx(ctx).
				Debug("Unable to extract avg_daily_updates agg for subscription", slog.String("feed_id", feedID))
			continue
		}
		stats[feedID] = float64(*updatesResult.Value)
	}

	return stats, nil
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
func (f *Feed) GetImage() *types.ImageInfo {
	return f.Image
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
	if err := elastic.UpdateDoc(ctx, schema.FeedsIndexRW, f.GetID(), map[string]any{
		"update_interval": f.UpdateInterval,
	}); err != nil {
		return fmt.Errorf("set feed update interval: %w", err)
	}

	return nil
}

// NewFeedFromURL generates a new Feed object from the given URL. If there is a problem generating the object, a non-nil
// error is returned.
func NewFeedFromURL(ctx context.Context, url string) (*Feed, error) {
	var feed *Feed

	ctx, cancel := context.WithTimeout(ctx, feeds.DefaultRequestTimeout)
	defer cancel()

	result, err := feeds.NewFeedFromURL(ctx, url)
	if err != nil {
		var validateErrs validator.ValidationErrors
		if errors.As(err, &validateErrs) {
			slogctx.FromCtx(ctx).Warn("Feed is invalid, continuing without validation",
				slog.String("url", url),
			)
			// On validation errors, try again without validation.
			var (
				err     error
				invalid *feeds.Feed
			)
			invalid, err = feeds.NewFeedFromURL(ctx, url, feeds.PerformValidation(false))
			if err != nil {
				return nil, fmt.Errorf("could not create feed from URL %s: %w", url, err)
			}
			result = invalid
		} else {
			return nil, fmt.Errorf("could not create feed from URL %s: %w", url, err)
		}
	}
	if result.GetImage() == nil {
		if err := feeds.FindFeedImage(ctx, result); err != nil {
			slogctx.FromCtx(ctx).WarnContext(ctx, "No image for feed.",
				slog.String("feed", result.GetTitle()),
				slog.String("url", result.GetSourceURL()),
			)
		}
	}
	feed = NewSyndicationFeed(url, result)

	return feed, nil
}

// NewSyndicationFeed converts the raw types.FeedSource into a Feed object.
func NewSyndicationFeed(url string, source *feeds.Feed) *Feed {
	id := "feed_" + strconv.FormatUint(xxhash.Sum64String(source.GetLink()), 10)
	feed := &Feed{
		FeedID:       id,
		CreatedAt:    time.Now().UTC(),
		LastFetched:  types.UnixEpoch,
		Published:    source.GetPublishedDate().UTC(),
		Updated:      source.GetUpdatedDate().UTC(),
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
	// Add the url used to find the feed to the source URLs if needed.
	if !slices.Contains(feed.SourceURLs, url) {
		feed.SourceURLs = append(feed.SourceURLs, url)
	}
	// Add any image found.
	if source.GetImage() != nil {
		feed.Image = source.GetImage()
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

func newFeedSortOptions(sort *Sort) []estypes.SortCombinationsVariant {
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

func newFeedSortCombinations(sort *Sort) []estypes.SortCombinations {
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
func FeedURLParser(ctx context.Context, urlStr string) (*url.URL, error) {
	// Parse the URL.
	slogctx.FromCtx(ctx).Debug("Parsing url", slog.String("url", urlStr))
	feedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	// For some popular sites that have an API or special URL for feeds, handle those.
	switch {
	case strings.Contains(feedURL.Host, "reddit.com") &&
		(!strings.HasPrefix(feedURL.Path, ".rss") || !strings.HasPrefix(feedURL.Path, ".rss/")):
		// Reddit can usually support a feed by appending `.rss` to the end of the subreddit URL.
		var err error
		if feedURL.Path, err = url.JoinPath(feedURL.Path, "/.rss"); err != nil {
			slogctx.FromCtx(ctx).Warn("Could not create subreddit RSS url.",
				slog.Any("err", err),
			)
		}
	case strings.HasSuffix(feedURL.Host, "tumblr.com") && (!strings.HasPrefix(feedURL.Path, "feed") || !strings.HasPrefix(feedURL.Path, "feed/")):
		// Tumblr blogs usually have their feed at the "/feed" path.
		var err error
		if feedURL.Path, err = url.JoinPath(feedURL.Path, "/feed"); err != nil {
			slogctx.FromCtx(ctx).Warn("Could not create create canonical Tumblr RSS url.",
				slog.Any("err", err),
			)
		}
	}

	return feedURL, nil
}
