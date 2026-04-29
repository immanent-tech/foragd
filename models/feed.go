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
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/calendarinterval"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/sortorder"
	"github.com/go-playground/validator/v10"
	feeds "github.com/immanent-tech/go-syndication"
	"github.com/immanent-tech/go-syndication/types"
	slogctx "github.com/veqryn/slog-context"
	"github.com/zeebo/xxh3"

	"github.com/immanent-tech/foragd/client"

	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/aggregations"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/results"
	"github.com/immanent-tech/foragd/reverseproxy"
)

// GetFeedByID fetches the given Feed by its id.
func GetFeedByID(ctx context.Context, id FeedID) (*Feed, error) {
	feed, err := elastic.GetDoc[FeedID, *Feed](ctx, schema.FeedsIndexRO, id)
	if err != nil {
		return nil, fmt.Errorf("get feed by id: %w", err)
	}
	return feed, nil
}

// AddFeed adds the given feed.
func AddFeed(ctx context.Context, feed *Feed) error {
	if err := elastic.CreateDoc(ctx, schema.FeedsIndexRW, feed.GetID(), feed); err != nil {
		return fmt.Errorf("add feed: %w", err)
	}
	return nil
}

// UpdateFeed applies the given updates to a Feed.
func UpdateFeed(ctx context.Context, id FeedID, updates map[string]any) error {
	if err := elastic.UpdateDoc(ctx, schema.SchedulerIndexRW, id, updates); err != nil {
		return fmt.Errorf("update feed: %w", err)
	}
	return nil
}

// BulkImportFeeds handles processing any number of NewFeedSubscriptionRequest requests.
func BulkImportFeeds(ctx context.Context, requests ...FeedSubscriptionRequest) []FeedSubscriptionResult {
	// Process requests.
	resultsCh := make(chan FeedSubscriptionResult)
	var wg sync.WaitGroup

	for request := range slices.Values(requests) {
		wg.Go(func() {
			// Find an existing or create a new feed from the requested URL.
			feed, isNew, err := FindOrCreateFeed(ctx, request.URL)
			if err != nil {
				resultsCh <- FeedSubscriptionResult{
					Request: &request,
					Error: &APIError{
						InternalError: fmt.Errorf("create subscription: %w", err),
						StatusCode:    http.StatusInternalServerError,
						UserMessage: NewErrorMessage(
							"Unable to create subscription",
							fmt.Sprintf("Could not find feed data for URL: %q", request.URL),
						),
					},
				}
				return
			}
			if isNew {
				// Add the feed if it is new.
				if err := AddFeed(ctx, feed); err != nil {
					resultsCh <- FeedSubscriptionResult{
						Request: &request,
						Error: &APIError{
							InternalError: fmt.Errorf("create subscription: %w", err),
							StatusCode:    http.StatusInternalServerError,
							UserMessage: NewErrorMessage(
								"Unable to add feed subscription",
								fmt.Sprintf("Could not create a feed for %s (%s)", feed.GetTitle(), request.URL),
							),
						},
					}
					return
				}
			}

			// Check for an existing subscription.
			subscription, err := GetSubscriptionByFeedID(ctx, feed.GetID())
			if err != nil && HTTPStatus(err) != http.StatusNotFound {
				resultsCh <- FeedSubscriptionResult{
					Request: &request,
					Error: &APIError{
						InternalError: fmt.Errorf("create subscription: %w", err),
						StatusCode:    http.StatusInternalServerError,
						UserMessage: NewErrorMessage(
							"Unable to create subscription",
							fmt.Sprintf("Could not determine existing subscription status for %s (%s)", feed.GetTitle(), request.URL),
						),
					},
				}
				return
			}
			if subscription != nil {
				resultsCh <- FeedSubscriptionResult{
					Request: &request,
					Error: &APIError{
						InternalError: errors.New("create subscription: already subscribed"),
						StatusCode:    http.StatusConflict,
						UserMessage: NewWarningMessage(
							"Already subscribed to feed",
							fmt.Sprintf("%s (%s)", feed.GetTitle(), request.URL),
						),
					},
				}
				return
			}

			// Create feed subscription.
			subscription, err = NewFeedSubscription(ctx, feed, nil)
			if err != nil {
				resultsCh <- FeedSubscriptionResult{
					Request: &request,
					Error: &APIError{
						InternalError: fmt.Errorf("create subscription: %w", err),
						StatusCode:    http.StatusInternalServerError,
						UserMessage: NewErrorMessage(
							"Unable to add subscription",
							fmt.Sprintf("Could create subscription data for feed %s (%s)", feed.GetTitle(), request.URL),
						),
					},
				}
				return
			}
			if err := AddSubscriptions(ctx, subscription); err != nil {
				resultsCh <- FeedSubscriptionResult{
					Request: &request,
					Error: &APIError{
						InternalError: fmt.Errorf("add subscription: %w", err),
						StatusCode:    http.StatusInternalServerError,
						UserMessage: NewErrorMessage(
							"Unable to add subscription",
							fmt.Sprintf("Could subscribe to feed %s (%s)", feed.GetTitle(), request.URL),
						),
					},
				}
				return
			}
			resultsCh <- FeedSubscriptionResult{
				Request:      &request,
				Subscription: subscription,
			}
		})
	}
	// Wait for all request processing to complete.
	go func() {
		defer close(resultsCh)
		wg.Wait()
	}()
	results := make([]FeedSubscriptionResult, 0, len(requests))
	// Gather results.
	for result := range resultsCh {
		results = append(results, result)
	}

	return results
}

// GetID retrieves (generates) a unique ID for a FeedStatus object.
func (s *FeedStatus) GetID() string {
	return strconv.FormatUint(xxh3.Hash([]byte(s.FeedID+s.Timestamp.String())), 10)
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
		query.Terms("feed_id", ids),
		len(ids),
		elastic.WithCollapseField("feed_id"),
		elastic.WithSort(NewItemSortOptions(&sort)...),
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
			query.Terms("feed_id", ids),
			// Must be published within last month.
			query.Bool(
				query.Should(
					query.Since("published", time.Now().UTC().Add(-24*30*time.Hour)),
					query.Since("updated", time.Now().UTC().Add(-24*30*time.Hour)),
				),
			),
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

// GetFeedLatestItems fetches the most recent count items for each given feed.
func GetFeedLatestItems(ctx context.Context, count int, feeds Feeds) (map[FeedID]Items, error) {
	queryResult, err := ItemsAggregation(ctx,
		query.Bool(
			query.Filter(
				query.Terms("feed_id", feeds.GetIDs()),
			),
		),
		0,
		aggregations.Aggs{
			"feed": estypes.Aggregations{
				Terms: &estypes.TermsAggregation{
					Field: new("feed_id"),
					Size:  new(len(feeds)),
				},
				Aggregations: map[string]estypes.Aggregations{
					"latest_items": {
						TopHits: &estypes.TopHitsAggregation{
							Size: &count,
							Sort: NewItemSortCombinations(new(SortNewestFirst)),
						},
					},
				},
			},
		})
	if err != nil {
		return nil, fmt.Errorf("fetch latest articles: %w", err)
	}
	feedsLatestItems := make(map[FeedID]Items)
	var wg sync.WaitGroup
	var mu sync.Mutex
	// Extract the feed aggregation.
	feedsAgg, err := aggregations.ExtractAggregation[*estypes.StringTermsAggregate](
		queryResult.Aggregations,
		"feed",
	)
	if err != nil {
		return nil, fmt.Errorf("extract feed aggregation: %w", err)
	}
	// Loop over the feed buckets.
	feedBuckets, err := aggregations.ExtractBuckets[estypes.StringTermsBucket](feedsAgg.Buckets)
	if err != nil {
		return nil, fmt.Errorf("extract feed aggregation buckets: %w", err)
	}
	for bucket := range slices.Values(feedBuckets) {
		if feedID, ok := bucket.Key.(FeedID); ok {
			wg.Go(func() {
				// Get the subscription with this feedID.
				feed := feeds.FindByID(feedID)
				if feed == nil {
					slogctx.FromCtx(ctx).
						Warn("Could not match feed in aggregation result to a subscription.",
							slog.String("feed_id", feedID),
						)
					return
				}
				// Extract the latest articles aggregation.
				latestItemsAggs, err := aggregations.ExtractAggregation[*estypes.TopHitsAggregate](
					bucket.Aggregations,
					"latest_items",
				)
				if err != nil {
					slogctx.FromCtx(ctx).Warn("Could not extract aggregation.",
						slog.String("aggregation", "latest_items"),
						slog.Any("error", err),
					)
					return
				}
				var (
					items Items
				)

				// Extract the latest items.
				//
				// * Note that the "latest_items" aggregation applies _source filtering,
				// * so only the given fields will be populated in the models.Item object.
				items, _, err = results.ExtractSourceFromHits[Item](latestItemsAggs.Hits.Hits)
				if err != nil {
					slogctx.FromCtx(ctx).
						Warn("Unable to extract latest articles from aggregations.",
							slog.Any("error", err),
						)
					return
				}
				// // Generate articles.
				// articles, err := models.GenerateArticles(ctx, items)
				// if err != nil {
				// 	slogctx.FromCtx(ctx).
				// 		Warn("Unable to generate articles from items.",
				// 			slog.Any("error", err),
				// 		)
				// 	return
				// }
				mu.Lock()
				feedsLatestItems[feed.GetID()] = items
				mu.Unlock()
			})
		}
	}

	wg.Wait()
	return feedsLatestItems, nil
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
	if err := elastic.UpdateDoc(ctx, schema.FeedsIndexRW, f.GetID(), map[string]any{
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
	existingFeeds, _, err := elastic.Search[*Feed](ctx,
		schema.FeedsIndexRO,
		query.Bool(
			query.Filter(
				query.Bool(
					query.Should(terms...),
				),
			),
		),
		1,
	)
	if err != nil {
		return nil, false, fmt.Errorf("search existing feeds: %w", err)
	}
	if len(existingFeeds) == 1 {
		// If an existing feed is found, use that feed.
		return existingFeeds[0], false, nil
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
