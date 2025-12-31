// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	estypes "github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/calendarinterval"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/sortorder"
	feeds "github.com/immanent-tech/go-syndication"
	"github.com/immanent-tech/go-syndication/types"
	"github.com/spaolacci/murmur3"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/aggregations"
	"github.com/immanent-tech/foragd/providers/elastic/query"
)

func getFeedUnreadCounts(
	ctx context.Context,
	subscriptions Subscriptions,
) (map[FeedID]int64, error) {
	// Retrieve user object.
	user, err := UserFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to get subscription unread counts: %w", err)
	}
	// Generate unread count query.
	subscriptionQueries := make([]query.Option, 0, len(subscriptions))
	for subscription := range slices.Values(subscriptions) {
		if subscription.GetSubscriptionType() != SubscriptionTypeFeed {
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
		ItemsIndexRO,
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
			if err := feeds.FindFeedImage(ctx, result.Feed); err != nil {
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
	// Generate an ID using a murmur hash of the feed's website URL.
	id := strings.Join(
		[]string{"feed_", strconv.FormatUint(murmur3.Sum64([]byte(source.GetLink())), 10)},
		"_",
	)
	feed := &Feed{
		FeedID:       id,
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
	// Add any image found.
	if source.GetImage() != nil {
		feed.Image = *source.GetImage()
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
