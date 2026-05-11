// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	estypes "github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/calendarinterval"
	"github.com/maypok86/otter/v2"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/results"
	"github.com/immanent-tech/foragd/providers/google/youtube"
)

var feedCache = otter.Must(&otter.Options[models.FeedID, models.Feed]{
	MaximumSize: 10_000,
})

// loadFeed will fetch the feed from Elasticsearch and cache it before returning the feed details.
func loadFeed(ctx context.Context, id models.FeedID) (models.Feed, error) {
	feed, err := elastic.GetDoc[models.FeedID, models.Feed](ctx, schema.FeedsIndexRO(), id)
	if err != nil {
		return models.Feed{}, fmt.Errorf("%w: %w", otter.ErrNotFound, err)
	}
	return feed, nil
}

// GetFeed retrieves a feed with the given FeedID.
func GetFeed(ctx context.Context, id models.FeedID) (*models.Feed, error) {
	switch feed, err := feedCache.Get(ctx, id, otter.LoaderFunc[models.FeedID, models.Feed](loadFeed)); {
	case err != nil && !errors.Is(err, elastic.ErrNotFound):
		return nil, fmt.Errorf("get feed: %w", err)
	case errors.Is(err, elastic.ErrNotFound):
		return nil, models.ErrNotFound
	default:
		return &feed, nil
	}
}

// GetFeeds retrieves the Feeds matching the given FeedIDs. It will fetch any cached versions before fetching from
// Elasticsearch (and then caching those).
func GetFeeds(ctx context.Context, ids ...models.FeedID) (models.Feeds, error) {
	var (
		feeds    models.Feeds
		err      error
		unCached []models.FeedID
	)

	// Fetch feeds from cache.
	for id := range slices.Values(ids) {
		if feed, found := feedCache.GetIfPresent(id); found {
			feeds = append(feeds, &feed)
		} else {
			unCached = append(unCached, id)
		}
	}
	// If there are feeds missing from the cache, fetch and cache them.
	if len(unCached) > 0 {
		feeds, err = elastic.GetDocs[models.FeedID, *models.Feed](ctx, schema.FeedsIndexRO(), ids...)
		if err != nil {
			return nil, fmt.Errorf("get items: %w", err)
		}
		for feed := range slices.Values(feeds) {
			feeds = append(feeds, feed)
			feedCache.Set(feed.GetID(), *feed)
		}
	}
	return feeds, nil
}

// AddFeed adds a new feed to Elasticsearch and the cache.
func AddFeed(ctx context.Context, feed *models.Feed) error {
	if err := elastic.CreateDoc(ctx, schema.FeedsIndexRW(), feed.GetID(), feed); err != nil {
		return fmt.Errorf("add feed: %w", err)
	}
	if _, ok := feedCache.Set(feed.GetID(), *feed); !ok {
		slogctx.FromCtx(ctx).Warn("Unable to cache new feed.",
			slog.String("feed_id", feed.GetID()),
		)
	}
	return nil
}

// UpdateFeed applies the given updates to a Feed. Any cached version of the feed is invalidated.
func UpdateFeed(ctx context.Context, id models.FeedID, updates map[string]any) error {
	if err := elastic.UpdateDoc(ctx, schema.SchedulerIndexRW(), id, updates, elastic.WithRefresh(true)); err != nil {
		return fmt.Errorf("update feed: %w", err)
	}
	feedCache.Invalidate(id)
	return nil
}

// BulkImportFeeds handles processing any number of NewFeedSubscriptionRequest requests.
func BulkImportFeeds(ctx context.Context, requests ...models.FeedSubscriptionRequest) []models.FeedSubscriptionResult {
	// Process requests.
	resultsCh := make(chan models.FeedSubscriptionResult)
	var wg sync.WaitGroup

	for request := range slices.Values(requests) {
		wg.Go(func() {
			// Find an existing or create a new feed from the requested URL.
			feed, isNew, err := models.FindOrCreateFeed(ctx, request.URL)
			if err != nil {
				resultsCh <- models.FeedSubscriptionResult{
					Request: &request,
					Error: &models.APIError{
						InternalError: fmt.Errorf("create subscription: %w", err),
						StatusCode:    http.StatusInternalServerError,
						UserMessage: models.NewErrorMessage(
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
					resultsCh <- models.FeedSubscriptionResult{
						Request: &request,
						Error: &models.APIError{
							InternalError: fmt.Errorf("create subscription: %w", err),
							StatusCode:    http.StatusInternalServerError,
							UserMessage: models.NewErrorMessage(
								"Unable to add feed subscription",
								fmt.Sprintf("Could not create a feed for %s (%s)", feed.GetTitle(), request.URL),
							),
						},
					}
					return
				}
			}

			// Check for an existing existingSubscriptions.
			existingSubscriptions, err := GetSubscriptionsByFeedID(ctx, feed.GetID())
			if err != nil && models.HTTPStatus(err) != http.StatusNotFound {
				resultsCh <- models.FeedSubscriptionResult{
					Request: &request,
					Error: &models.APIError{
						InternalError: fmt.Errorf("create subscription: %w", err),
						StatusCode:    http.StatusInternalServerError,
						UserMessage: models.NewErrorMessage(
							"Unable to create subscription",
							fmt.Sprintf("Could not determine existing subscription status for %s (%s)", feed.GetTitle(), request.URL),
						),
					},
				}
				return
			}
			if existingSubscriptions != nil {
				resultsCh <- models.FeedSubscriptionResult{
					Request: &request,
					Error: &models.APIError{
						InternalError: errors.New("create subscription: already subscribed"),
						StatusCode:    http.StatusConflict,
						UserMessage: models.NewWarningMessage(
							"Already subscribed to feed",
							fmt.Sprintf("%s (%s)", feed.GetTitle(), request.URL),
						),
					},
				}
				return
			}

			// Create feed newSubscription.
			newSubscription, err := models.NewFeedSubscription(ctx, feed, nil)
			if err != nil {
				resultsCh <- models.FeedSubscriptionResult{
					Request: &request,
					Error: &models.APIError{
						InternalError: fmt.Errorf("create subscription: %w", err),
						StatusCode:    http.StatusInternalServerError,
						UserMessage: models.NewErrorMessage(
							"Unable to add subscription",
							fmt.Sprintf("Could create subscription data for feed %s (%s)", feed.GetTitle(), request.URL),
						),
					},
				}
				return
			}
			if err := AddSubscriptions(ctx, newSubscription); err != nil {
				resultsCh <- models.FeedSubscriptionResult{
					Request: &request,
					Error: &models.APIError{
						InternalError: fmt.Errorf("add subscription: %w", err),
						StatusCode:    http.StatusInternalServerError,
						UserMessage: models.NewErrorMessage(
							"Unable to add subscription",
							fmt.Sprintf("Could subscribe to feed %s (%s)", feed.GetTitle(), request.URL),
						),
					},
				}
				return
			}
			resultsCh <- models.FeedSubscriptionResult{
				Request:      &request,
				Subscription: newSubscription,
			}
		})
	}
	// Wait for all request processing to complete.
	go func() {
		defer close(resultsCh)
		wg.Wait()
	}()
	results := make([]models.FeedSubscriptionResult, 0, len(requests))
	// Gather results.
	for result := range resultsCh {
		results = append(results, result)
	}

	return results
}

// GetFeedLatestItems fetches the most recent count items for each given feed.
func GetFeedLatestItems(
	ctx context.Context,
	count int,
	feedIDs ...models.FeedID,
) (map[models.FeedID]models.Items, error) {
	resp, err := elastic.Search[*models.Item](ctx,
		schema.ItemsIndexRO(),
		query.Bool(
			query.Filter(
				query.Terms("feed_id", feedIDs),
			),
		),
		elastic.WithAggregations(
			elastic.Aggs{
				"feed": estypes.Aggregations{
					Terms: &estypes.TermsAggregation{
						Field: new("feed_id"),
						Size:  new(len(feedIDs)),
					},
					Aggregations: map[string]estypes.Aggregations{
						"latest_items": {
							TopHits: &estypes.TopHitsAggregation{
								Size: &count,
								Sort: NewItemSortCombinations(new(models.SortNewestFirst)),
							},
						},
					},
				},
			},
		),
		elastic.WithSize(0),
		elastic.WithDocSorting(),
	)
	if err != nil {
		return nil, fmt.Errorf("fetch latest articles: %w", err)
	}
	feedsLatestItems := make(map[models.FeedID]models.Items)
	var wg sync.WaitGroup
	var mu sync.Mutex
	// Extract the feed aggregation.
	feedsAgg, hasFeedAgg, err := elastic.ExtractAggregation[*estypes.StringTermsAggregate](
		resp.Aggregations,
		"feed",
	)
	if !hasFeedAgg || err != nil {
		return nil, fmt.Errorf("extract feed aggregation: %w", err)
	}
	// Loop over the feed buckets.
	feedBuckets, err := elastic.ExtractBuckets[estypes.StringTermsBucket](feedsAgg.Buckets)
	if err != nil {
		return nil, fmt.Errorf("extract feed aggregation buckets: %w", err)
	}
	for bucket := range slices.Values(feedBuckets) {
		if feedID, ok := bucket.Key.(models.FeedID); ok {
			wg.Go(func() {
				// Get the subscription with this feedID.
				if !slices.Contains(feedIDs, feedID) {
					slogctx.FromCtx(ctx).
						Warn("Could not match feed in aggregation result to a subscription.",
							slog.String("feed_id", feedID),
						)
					return
				}
				// Extract the latest articles aggregation.
				latestItemsAggs, hasLatestItemsAgg, err := elastic.ExtractAggregation[*estypes.TopHitsAggregate](
					bucket.Aggregations,
					"latest_items",
				)
				if !hasLatestItemsAgg || err != nil {
					slogctx.FromCtx(ctx).Warn("Could not extract aggregation.",
						slog.String("aggregation", "latest_items"),
						slog.Any("error", err),
					)
					return
				}
				var (
					items models.Items
				)

				// Extract the latest items.
				//
				// * Note that the "latest_items" aggregation applies _source filtering,
				// * so only the given fields will be populated in the models.Item object.
				items, _, err = results.ExtractSourceFromHits[*models.Item](latestItemsAggs.Hits.Hits)
				if err != nil {
					slogctx.FromCtx(ctx).
						Warn("Unable to extract latest articles from elastic.",
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
				feedsLatestItems[feedID] = items
				mu.Unlock()
			})
		}
	}

	wg.Wait()
	return feedsLatestItems, nil
}

// SuggestYoutubeFeeds will return a list of youtube feeds that match the given text.
func SuggestYoutubeFeeds(ctx context.Context, text string) (*models.FeedSuggestionsResults, error) {
	// Retrieve the user object.
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}

	// Get user subscriptions.
	subscriptions, err := GetAllSubscriptions(ctx, user)
	if err != nil && !errors.Is(err, models.ErrNotFound) {
		return nil, fmt.Errorf("get subscriptions: %w", err)
	}

	// Perform a search on youtube to find a channel that matches the user's query.
	channelResults, err := youtube.FindChannels(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("search youtube channels: %w", err)
	}
	// Extract the channel RSS feed urls.
	urls := make([]string, 0, len(channelResults))
	for channel := range slices.Values(channelResults) {
		urls = append(urls, "https://www.youtube.com/feeds/videos.xml?channel_id="+channel.ID)
	}
	var feeds models.Feeds
	// Try to find existing feeds that match the query.
	resp, err := elastic.Search[*models.Feed](
		ctx,
		schema.FeedsIndexRO(),
		query.Bool(
			query.MustNot(
				// User must not already be subscribed.
				query.Terms("feed_id", subscriptions.GetFeedIDs()),
			),
			query.Should(
				// Match source_urls (preferred) or url.
				query.Terms(
					"source_urls",
					urls,
					query.WithQueryBoost[*query.TermsQuery](10.0),
				),
				query.Terms(
					"url",
					urls,
				),
			),
		),
		elastic.WithSort(
			models.NewFeedSortOptions(new(models.SortMostRelevant))...,
		),
		elastic.WithSize(5),
	)
	if err != nil {
		return nil, fmt.Errorf("search feeds: %w", err)
	}

	// If existing feeds are found, return the results
	if len(resp.Results) > 0 {
		feeds = resp.Results
		// Retrieve the latest 3 articles for each feed.
		latestItems, err := GetFeedLatestItems(ctx, 3, feeds.GetIDs()...)
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Unable to get latest items for feeds.",
				slog.Any("error", err),
			)
		}
		return &models.FeedSuggestionsResults{
			Text:        text,
			Feeds:       feeds,
			LatestItems: latestItems,
		}, nil
	}

	// Try to create new feeds for the urls.
	latestItems := make(map[models.FeedID]models.Items)
	for url := range slices.Values(urls) {
		slogctx.FromCtx(ctx).Debug("Looking for new feed for URL.",
			slog.String("url", url),
		)
		newFeed, err := models.NewFeedFromURL(ctx, url, "", false)
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Unable to find feed at URL.",
				slog.String("url", url),
				slog.Any("error", err),
			)
			continue
		}
		// Retrieve the latest 3 articles for each feed.
		if items := newFeed.GetItems(); len(items) > 0 {
			// Truncate to 3 items.
			if len(items) > 3 {
				items = items[:3]
			}
			latestItems[newFeed.GetID()] = items
		}
		feeds = append(feeds, newFeed)
	}
	return &models.FeedSuggestionsResults{
		Text:        text,
		Feeds:       feeds,
		LatestItems: latestItems,
	}, nil
}

// SuggestFeeds returns a feeds and their latest articles that match the given text. It will search first for existing
// feeds in Elasticsearch. If the given text is a URL, it will fallback to searching the website for a feed.
func SuggestFeeds(ctx context.Context, text string) (*models.FeedSuggestionsResults, error) {
	// Retrieve the user object.
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}

	// Get user subscriptions.
	subscriptions, err := GetAllSubscriptions(ctx, user)
	if err != nil && !errors.Is(err, models.ErrNotFound) {
		return nil, fmt.Errorf("get subscriptions: %w", err)
	}

	// Find the top 5 feeds that match the user's query and which they are not already subscribed to.
	var feedSearchQuery query.Option
	switch {
	case strings.HasPrefix(text, "http"):
		feedSearchQuery = query.Bool(
			query.MustNot(
				// User must not already be subscribed.
				query.Terms("feed_id", subscriptions.GetFeedIDs()),
			),
			query.Should(
				// For URLs, match with and without a trailing slash. Boost source_urls over url.
				query.Term(
					"source_urls",
					strings.TrimSuffix(text, "/"),
					query.WithQueryBoost[*query.TermQuery](10.0),
				),
				query.Term(
					"url",
					strings.TrimSuffix(text, "/"),
					query.WithQueryBoost[*query.TermQuery](5.0),
				),
				query.Term("source_urls", text, query.WithQueryBoost[*query.TermQuery](10.0)),
				query.Term("url", text, query.WithQueryBoost[*query.TermQuery](5.0)),
				// Wildcard URL prefix.
				query.Wildcard("source_urls", text+"*"),
				query.Wildcard("url", text+"*"),
			),
		)
	default:
		feedSearchQuery = query.Bool(
			query.MustNot(
				// User must not already be subscribed.
				query.Terms("feed_id", subscriptions.GetFeedIDs()),
			),
			query.Should(
				// Exact match text on title with significant boost.
				query.Term(
					"title.exact",
					text,
					query.WithQueryBoost[*query.TermQuery](10.0),
				),
				// Title or description contains text, with boost for title.
				query.MultiMatch(
					text,
					[]string{"title^5", "description"},
					query.WithFuzziness[*query.MultiMatchQuery]("AUTO"),
				),
				// Match phrase in description.
				query.MatchPhrase(
					"description",
					text,
				),
				// Match an existing subscription category.
				query.Term("categories", text),
			),
		)
	}

	// Try to find existing feeds that match the query.
	resp, err := elastic.Search[*models.Feed](
		ctx,
		schema.FeedsIndexRO(),
		feedSearchQuery,
		elastic.WithSort(
			models.NewFeedSortOptions(new(models.SortMostRelevant))...,
		),
		elastic.WithSize(5),
	)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to find feed suggestions.",
			slog.Any("error", err),
		)
	}

	var feeds models.Feeds
	feeds = resp.Results

	if len(feeds) > 0 {
		// Handle matching feeds.
		if len(feeds) > 0 {
			// Retrieve the latest 3 articles for each feed.
			latestItems, err := GetFeedLatestItems(ctx, 3, feeds.GetIDs()...)
			if err != nil {
				slogctx.FromCtx(ctx).Warn("Unable to get latest items for feeds.",
					slog.Any("error", err),
				)
			}
			return &models.FeedSuggestionsResults{
				Text:        text,
				Feeds:       feeds,
				LatestItems: latestItems,
			}, nil
		}
	}

	// If no matching feeds but the query is a valid URL, try to find a feed at the URL.
	if strings.HasPrefix(text, "http") {
		if newFeedURL, err := url.Parse(text); err == nil {
			slogctx.FromCtx(ctx).Debug("Looking for new feed for URL.",
				slog.String("url", newFeedURL.String()),
			)
			newFeed, err := models.NewFeedFromURL(ctx, newFeedURL.String(), "", false)
			if err != nil {
				return nil, fmt.Errorf("new feed from url: %w", err)
			}
			latestItems := make(map[models.FeedID]models.Items)
			if items := newFeed.GetItems(); len(items) > 0 {
				// Truncate to 3 items.
				if len(items) > 3 {
					items = items[:3]
				}
				latestItems[newFeed.GetID()] = items
			}
			return &models.FeedSuggestionsResults{
				Text:        text,
				Feeds:       models.Feeds{newFeed},
				LatestItems: latestItems,
			}, nil
		}
	}

	// No results, send empty set.
	return &models.FeedSuggestionsResults{
		Text:        text,
		LatestItems: make(map[models.FeedID]models.Items),
	}, nil
}

func getFeedUnreadCounts(
	ctx context.Context,
	subscriptions models.Subscriptions,
) (map[models.FeedID]int64, error) {
	// Retrieve user object.
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}

	// Generate unread count query.
	subscriptionQueries := make([]query.Option, 0, len(subscriptions))
	for subscription := range slices.Values(subscriptions) {
		if subscription.GetSubscriptionType() != models.SubscriptionTypeFeed &&
			subscription.GetSubscriptionType() != models.SubscriptionTypeEmail {
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
	// Build elastic.
	termsField := "feed_id"
	termsCount := len(subscriptions)
	aggs := elastic.Aggs{
		"UnreadCounts": estypes.Aggregations{
			Terms: &estypes.TermsAggregation{
				Field: &termsField,
				Size:  &termsCount,
			},
		},
	}
	// Perform aggregation.
	resp, err := elastic.Search[*models.Item](ctx,
		schema.ItemsIndexRO(),
		query,
		elastic.WithAggregations(aggs),
		elastic.WithSize(0),
		elastic.WithDocSorting(),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to get subscription unread counts: %w", err)
	}

	unreadCounts, ok := resp.Aggregations["UnreadCounts"].(*estypes.StringTermsAggregate)
	if !ok {
		return nil, fmt.Errorf(
			"unable to get feed stats: UnreadCounts aggregations invalid: %w",
			models.ErrInvalidAPIResult,
		)
	}
	unreadCountsBuckets, ok := unreadCounts.Buckets.([]estypes.StringTermsBucket)
	if !ok {
		return nil, fmt.Errorf(
			"unable to get feed stats: UnreadCounts aggregations invalid: %w",
			models.ErrInvalidAPIResult,
		)
	}

	stats := make(map[models.SubscriptionID]int64)

	// Loop through the aggregation results and extract the unread count for each feed.
	for feed := range slices.Values(unreadCountsBuckets) {
		var feedID models.FeedID
		if feedID, ok = feed.Key.(string); ok {
			stats[feedID] = feed.DocCount
		}
	}
	return stats, nil
}

func getFeedLastUpdates(ctx context.Context, ids ...models.FeedID) (map[models.FeedID]time.Time, error) {
	sort := models.SortNewestFirst
	resp, err := elastic.Search[*models.Item](
		ctx,
		schema.ItemsIndexRO(),
		query.Terms("feed_id", ids),
		elastic.WithSize(len(ids)),
		elastic.WithCollapseField("feed_id"),
		elastic.WithSort(NewItemSortOptions(&sort)...),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to get feed last updates: %w", err)
	}

	updates := make(map[models.FeedID]time.Time)

	for item := range slices.Values(resp.Results) {
		updates[item.GetFeedID()] = item.GetTimestamp()
	}

	return updates, nil
}

// GetFeedSubscriptionStats fetches the stats for FeedSubscriptions and returns a map of the SubscriptionID to
// SubscriptionStats that can be used to lookup the stats pertaining to a particular subscription.
func getFeedAverageDailyUpdates(ctx context.Context, ids ...models.FeedID) (map[models.FeedID]float64, error) {
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
	// Build elastic.
	termsField := "feed_id"
	termsCount := len(ids)
	dateHistoField := "@timestamp"
	dateFormat := "yyyy-MM-dd"
	aggs := elastic.Aggs{
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

	resp, err := elastic.Search[*models.Item](ctx,
		schema.ItemsIndexRO(),
		query,
		elastic.WithAggregations(aggs),
		elastic.WithSize(len(ids)),
		elastic.WithDocSorting(),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to get feed stats: Feed aggregation invalid: %w", models.ErrInvalidAPIResult)
	}
	feedStats, ok := resp.Aggregations["feed"].(*estypes.StringTermsAggregate)
	if !ok {
		return nil, fmt.Errorf("unable to get feed stats: Feed aggregation invalid: %w", models.ErrInvalidAPIResult)
	}
	feedStatsBuckets, ok := feedStats.Buckets.([]estypes.StringTermsBucket)
	if !ok {
		return nil, fmt.Errorf("unable to get feed stats: Feed aggregation invalid: %w", models.ErrInvalidAPIResult)
	}

	stats := make(map[models.FeedID]float64)

	// Loop through the aggregation results and extract the daily updates metric for each feed.
	for feed := range slices.Values(feedStatsBuckets) {
		var feedID models.FeedID
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
