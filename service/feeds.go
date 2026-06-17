// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package service

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
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
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/maypok86/otter/v2"
	slogctx "github.com/veqryn/slog-context"
	"github.com/zeebo/xxh3"

	feeds "github.com/immanent-tech/go-syndication"
	"github.com/immanent-tech/go-syndication/atom"
	"github.com/immanent-tech/go-syndication/opml"
	"github.com/immanent-tech/go-syndication/rss"
	"github.com/immanent-tech/go-syndication/types"
	"github.com/immanent-tech/go-syndication/validation"

	"github.com/immanent-tech/foragd/client"
	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/pkg/formats/html"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/results"
	"github.com/immanent-tech/foragd/providers/google/news"
	"github.com/immanent-tech/foragd/providers/google/youtube"
	"github.com/immanent-tech/foragd/providers/zyte"
)

var feedCache = otter.Must(&otter.Options[models.FeedID, *models.Feed]{
	MaximumSize: 10_000,
})

// loadFeed will fetch the feed from Elasticsearch and cache it before returning the feed details.
func loadFeed(ctx context.Context, id models.FeedID) (*models.Feed, error) {
	feed, err := elastic.GetDoc[models.FeedID, *models.Feed](ctx, schema.FeedsIndexRO(), id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", otter.ErrNotFound, err)
	}
	return feed, nil
}

// GetFeed retrieves a feed with the given FeedID.
func GetFeed(ctx context.Context, id models.FeedID) (*models.Feed, error) {
	switch feed, err := feedCache.Get(ctx, id, otter.LoaderFunc[models.FeedID, *models.Feed](loadFeed)); {
	case err != nil && !errors.Is(err, elastic.ErrNotFound):
		return nil, fmt.Errorf("get feed: %w", err)
	case errors.Is(err, elastic.ErrNotFound):
		return nil, models.ErrNotFound
	default:
		return feed, nil
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
			feeds = append(feeds, feed)
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
			feedCache.Set(feed.GetID(), feed)
		}
	}
	return feeds, nil
}

// AddFeed adds a new feed to Elasticsearch and the cache.
func AddFeed(ctx context.Context, feed *models.Feed) error {
	if err := elastic.CreateDoc(ctx, schema.FeedsIndexRW(), feed.GetID(), feed); err != nil {
		return fmt.Errorf("add feed: %w", err)
	}
	if _, ok := feedCache.Set(feed.GetID(), feed); !ok {
		slogctx.FromCtx(ctx).Warn("Unable to cache new feed.",
			slog.String("feed_id", feed.GetID()),
		)
	}
	return nil
}

// UpdateFeed applies the given updates to a Feed. Any cached version of the feed is invalidated.
func UpdateFeed(ctx context.Context, id models.FeedID, updates map[string]any) error {
	if err := elastic.UpdateDoc(ctx, schema.FeedsIndexRW(), id, updates, elastic.WithRefresh(true)); err != nil {
		return fmt.Errorf("update feed: %w", err)
	}
	feedCache.Invalidate(id)
	return nil
}

// UpdateFeedDetails takes a copy of a feed that has been recently fetched/refreshed and checks/updates various fields.
// If there are updates, these are then saved.
func UpdateFeedDetails(ctx context.Context, oldData, newData *models.Feed, lastFetched time.Time) error {
	if diff := cmp.Diff(*oldData, *newData,
		cmpopts.IgnoreFields(models.Feed{}, "Updated", "Published", "LastFetched", "CreatedAt"),
		cmpopts.EquateEmpty(),
		cmpopts.IgnoreUnexported(),
	); diff != "" {
		newData.LastFetched = lastFetched
		if _, err := elastic.BulkUpdate(ctx, schema.FeedsIndexRW(), newData); err != nil {
			return fmt.Errorf("update feed: %w", err)
		}
	} else {
		if err := UpdateFeed(ctx, newData.GetID(), map[string]any{
			"last_fetched": lastFetched,
		}); err != nil {
			return fmt.Errorf("update feed last_fetched: %w", err)
		}
	}

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
			feed, isNew, err := FindOrCreateFeed(ctx, request.URL)
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

			existingSubscriptions, err := GetAllSubscriptions(ctx)
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
			existingSubscriptions = existingSubscriptions.FilterByFeedIDs(feed.GetID())
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

// GetFeedLatestItems fetches the most recent count items for each given feed. An optional query clause can be specified
// that will be added to the bool filter clause of the query to apply additional filtering to the items.
func GetFeedLatestItems(
	ctx context.Context,
	count int,
	feedIDs []models.FeedID,
	extraQuery query.Option,
) (map[models.FeedID]models.Items, error) {
	resp, err := elastic.Search[*models.Item](ctx,
		schema.ItemsIndexRO(),
		query.Bool(
			query.Filter(
				query.Terms("feed_id", feedIDs),
				// extraQuery is ignored if nil.
				extraQuery,
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
				// Ensure proper sorting.
				items = items.SortByTimestamp()
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
	// Get user subscriptions.
	subscriptions, err := GetAllSubscriptions(ctx)
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
			NewFeedSortOptions(new(models.SortMostRelevant))...,
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
		latestItems, err := GetFeedLatestItems(ctx, 3, feeds.GetIDs(), nil)
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
		newFeed, err := FetchFeed(ctx, url)
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

// SuggestGoogleNewsFeeds will return a google news RSS feed for the given search query.
func SuggestGoogleNewsFeeds(ctx context.Context, text string) (*models.FeedSuggestionsResults, error) {
	newsURL, err := news.GenerateRSSURL(text)
	if err != nil {
		return nil, fmt.Errorf("generate news RSS URL: %w", err)
	}

	// Get user subscriptions.
	subscriptions, err := GetAllSubscriptions(ctx)
	if err != nil && !errors.Is(err, models.ErrNotFound) {
		return nil, fmt.Errorf("get subscriptions: %w", err)
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
				query.Term(
					"source_urls",
					newsURL.String(),
					query.WithQueryBoost[*query.TermQuery](10.0),
				),
				query.Term(
					"url",
					newsURL.String(),
				),
			),
		),
		elastic.WithSort(
			NewFeedSortOptions(new(models.SortMostRelevant))...,
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
		latestItems, err := GetFeedLatestItems(ctx, 3, feeds.GetIDs(), nil)
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
	slogctx.FromCtx(ctx).Debug("Looking for new feed for URL.",
		slog.String("url", newsURL.String()),
	)
	newFeed, err := FetchFeed(ctx, newsURL.String())
	if err != nil {
		return nil, fmt.Errorf("unable to fetch google news RSS feed: %w", err)
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
	return &models.FeedSuggestionsResults{
		Text:        text,
		Feeds:       feeds,
		LatestItems: latestItems,
	}, nil
}

// SuggestFeeds returns a feeds and their latest articles that match the given text. It will search first for existing
// feeds in Elasticsearch. If the given text is a URL, it will fallback to searching the website for a feed.
func SuggestFeeds(ctx context.Context, text string) (*models.FeedSuggestionsResults, error) {
	// Get user subscriptions.
	subscriptions, err := GetAllSubscriptions(ctx)
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
			NewFeedSortOptions(new(models.SortMostRelevant))...,
		),
		elastic.WithSize(5),
	)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to find feed suggestions.",
			slog.Any("error", err),
		)
	}

	if len(resp.Results) > 0 {
		// Retrieve the latest 3 articles for each feed.
		latestItems, err := GetFeedLatestItems(ctx, 3, models.Feeds(resp.Results).GetIDs(), nil)
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Unable to get latest items for feeds.",
				slog.Any("error", err),
			)
		}
		return &models.FeedSuggestionsResults{
			Text:        text,
			Feeds:       resp.Results,
			LatestItems: latestItems,
		}, nil
	}

	// If no matching feeds but the query is a valid URL, try to find a feed at the URL.
	if strings.HasPrefix(text, "http") {
		if newFeedURL, err := url.Parse(text); err == nil {
			slogctx.FromCtx(ctx).Debug("Looking for new feed for URL.",
				slog.String("url", newFeedURL.String()),
			)
			newFeed, err := FetchFeed(ctx, newFeedURL.String())
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

// GenerateOPML generates an OPML file of the feeds listed by the given IDs.
func GenerateOPML(ctx context.Context, feedIDs ...models.FeedID) ([]byte, error) {
	feeds, err := GetFeeds(ctx, feedIDs...)
	if err != nil {
		return nil, fmt.Errorf("get feeds: %w", err)
	}

	// Create outlines for all subscriptions.
	outlines := make([]opml.Outline, 0, len(feeds))
	for feed := range slices.Values(feeds) {
		outlines = append(
			outlines,
			*opml.NewSubscriptionOutline(feed.GetTitle(), feed.GetSourceURLs()[0],
				opml.WithHTMLURL(feed.GetLink()),
				opml.WithOutlineTitle(feed.GetTitle()),
			),
		)
	}
	// Generate the opml file from the outlines.
	title := config.AppName + " Export (" + time.Now().Format(time.DateTime) + ")"
	opmlExport := opml.NewOPML(
		opml.WithTitle(title),
		opml.WithOutlines(outlines...),
	)
	// Marshal the opml file and convert to a byte reader.
	data, err := xml.Marshal(opmlExport)
	if err != nil {
		return nil, fmt.Errorf("marshal opml: %w", err)
	}
	data = []byte(xml.Header + string(data))

	return data, nil
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
	// Perform aggregation.
	resp, err := elastic.Search[*models.Item](ctx,
		schema.ItemsIndexRO(),
		query.Bool(
			query.Filter(
				query.Bool(
					query.Should(subscriptionQueries...),
				),
			),
		),
		elastic.WithAggregations(
			elastic.Aggs{
				"UnreadCounts": estypes.Aggregations{
					Terms: &estypes.TermsAggregation{
						Field: new("feed_id"),
						Size:  new(len(subscriptions)),
					},
				},
			},
		),
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

// FetchOptions defines options that manipulate how a feed will be fetched from a remote URL.
type FetchOptions struct {
	Proxy  bool
	FeedID models.FeedID
}

// FetchOption is a functional option that controls some aspect of how a feed will be fetched.
type FetchOption func(*FetchOptions)

// FetchWithProxy option will ensure the fetch feed request is proxied.
func FetchWithProxy(value bool) FetchOption {
	return func(fo *FetchOptions) {
		fo.Proxy = value
	}
}

// FetchWithFeedID assigns the given FeedID to the feed object.
func FetchWithFeedID(id models.FeedID) FetchOption {
	return func(fo *FetchOptions) {
		fo.FeedID = id
	}
}

// FetchFeed retrieves the feed found at the given URL.
func FetchFeed(ctx context.Context, feedURL string, options ...FetchOption) (*models.Feed, error) {
	opts := &FetchOptions{}
	for option := range slices.Values(options) {
		option(opts)
	}

	// Parse the URL to ensure its valid.
	sourceURL, err := feedURLParser(feedURL)
	if err != nil {
		return nil, models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("parse url: %w", err))
	}

	// Add source-specific options.
	for extraOption := range slices.Values(addFetchOptions(sourceURL)) {
		extraOption(opts)
	}

	// Create a buffer for the feed data.
	feedBuf, ok := bufPool.Get().(*bytes.Buffer)
	if !ok {
		return nil, models.NewAPIError(http.StatusInternalServerError, errors.New("get feed buffer failed"))
	}
	feedBuf.Reset()
	defer bufPool.Put(feedBuf)

	// Fetch the feed data from the source url.
	var contentType string
	switch opts.Proxy {
	case false:
		slogctx.FromCtx(ctx).Debug("Fetching feed directly.",
			slog.String("feed_url", sourceURL.String()),
		)
		resp, err := client.Load().R().
			SetContext(ctx).
			SetDoNotParseResponse(true).
			// SetDebug(true).
			Get(sourceURL.String())
		switch {
		case err != nil:
			return nil, models.NewAPIError(http.StatusInternalServerError, err)
		case resp.IsError():
			if resp.StatusCode() == http.StatusForbidden || resp.StatusCode() == http.StatusTooManyRequests {
				slogctx.FromCtx(ctx).Debug("Potentially blocked. Retrying request through proxy.",
					slog.String("feed_url", sourceURL.String()),
				)
				return FetchFeed(ctx, feedURL,
					FetchWithProxy(true),
					FetchWithFeedID(opts.FeedID),
				)
			}
			return nil, models.NewAPIError(resp.StatusCode(), errors.New(resp.Status()))
		}
		defer resp.RawBody().Close()

		contentType = resp.Header().Get("Content-Type")

		if resp.Header().Get("Content-Encoding") == "gzip" {
			// For gzipped response, uncompress first.
			reader, err := gzip.NewReader(resp.RawBody())
			if err != nil {
				return nil, fmt.Errorf("read gzip response: %w", err)
			}
			defer reader.Close()
			const maxBodySize = 10 * 1024 * 1024 // 10 MB limit
			limitReader := io.LimitReader(reader, maxBodySize)
			if _, err := io.Copy(feedBuf, limitReader); err != nil {
				return nil, models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("read response: %w", err))
			}
		} else {
			// Read response directly.
			if _, err := io.Copy(feedBuf, resp.RawBody()); err != nil {
				return nil, models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("read response: %w", err))
			}
		}
	case true:
		slogctx.FromCtx(ctx).Debug("Fetching feed via proxy.",
			slog.String("feed_url", sourceURL.String()),
			slog.String("feed_id", opts.FeedID),
		)
		resp, err := zyte.Proxy(
			ctx,
			sourceURL.String(),
			zyte.WithResponseBody(true),
			zyte.WithFollowRedirects(true),
			zyte.WithTag("feed_id", opts.FeedID),
		)
		if err != nil {
			if zyteErr, isZyteErr := errors.AsType[*zyte.ResponseError](err); isZyteErr {
				return nil, models.NewAPIError(zyteErr.HTTPStatus(), fmt.Errorf("proxy request: %w", zyteErr))
			}
			return nil, models.NewAPIError(http.StatusInternalServerError, err)
		}
		body, err := resp.GetHTMLResponse()
		if err != nil {
			return nil, models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("get response body: %w", err))
		}
		if _, err := feedBuf.Write(body); err != nil {
			return nil, models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("read response: %w", err))
		}
	}

	// Parse the response as a feed type.
	var feedData *feeds.Feed
	switch {
	case bytes.Contains(feedBuf.Bytes(), []byte("<feed")):
		// Atom feed.
		feedData, err = feeds.NewFeedFromBytes[*atom.Feed](feedBuf.Bytes())
		if err != nil && errors.Is(err, &validation.StructError{}) {
			return nil, models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("parse atom: %w", err))
		}
	case bytes.Contains(feedBuf.Bytes(), []byte("<rss")):
		// RSS feed.
		feedData, err = feeds.NewFeedFromBytes[*rss.RSS](feedBuf.Bytes())
		if err != nil && errors.Is(err, &validation.StructError{}) {
			return nil, models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("parse rss: %w", err))
		}
	case bytes.Contains(feedBuf.Bytes(), []byte("<html")):
		// HTML webpage. Use "autodiscovery" to find feed.
		if newURL, err := html.DiscoverFeedURL(
			sourceURL,
			feedBuf.Bytes(),
		); err == nil && newURL != "" &&
			newURL != sourceURL.String() {
			slogctx.FromCtx(ctx).Debug("Found feed URL in HTML, re-fetching.")
			return FetchFeed(ctx, newURL, options...)
		}
		return nil, models.ErrNotFound
	default:
		return nil, models.NewAPIError(
			http.StatusUnsupportedMediaType,
			fmt.Errorf("unsupported media type: %s", contentType),
		)
	}

	// Handle getting through the switch but still not parsing the content.
	if feedData == nil {
		return nil, models.ErrNotFound
	}

	// If the source URL is not set, set it.
	if feedData.GetSourceURL() == "" || feedData.GetSourceURL() != sourceURL.String() {
		feedData.SetSourceURL(sourceURL.String())
	}

	feed := NewFeed(feedData.GetSourceURL(), opts.FeedID, feedData)

	// For Atom, assume a default hourly update.
	if feedData.SourceType == feeds.TypeAtom || feedData.SourceType == feeds.TypeJSONFeed {
		feed.UpdateInterval = int64(time.Hour)
	}

	// For RSS use either the reasonable interval given by the feed or a reasonable default.
	if feedData.SourceType == feeds.TypeRSS {
		switch interval := feedData.GetUpdateInterval(); {
		case interval < time.Minute:
			// Set really short update intervals to every 5 minutes.
			feed.UpdateInterval = int64(5 * time.Minute)
		case interval > 24*time.Hour:
			// Set really long update intervals to daily.
			feed.UpdateInterval = int64(24 * time.Hour)
		default:
			feed.UpdateInterval = int64(interval)
		}
	}

	// Set the method used to fetch the feed.
	if opts.Proxy {
		feed.FetchMethod = models.FeedFetchMethodProxied
	} else {
		feed.FetchMethod = models.FeedFetchMethodDirect
	}

	return feed, nil
}

// feedURLParser parses the given URL string into a url.URL object, applying some additional rules for known domains on
// where to find their feeds.
func feedURLParser(urlStr string) (*url.URL, error) {
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
		switch {
		case !strings.HasPrefix(feedURL.Path, "rss") && !strings.HasPrefix(feedURL.Path, "rss/"):
			// Tumblr blogs usually have their feed at the "/feed" path.
			var err error
			if feedURL.Path, err = url.JoinPath(feedURL.Path, "/rss"); err != nil {
				return nil, fmt.Errorf("generate RSS feed for tumblr.com URL: %w", err)
			}
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

// addFetchOptions returns fetch options that are source-specific. This will append or overide existing fetch options to
// ensure the feed can be fetched correctly.
func addFetchOptions(feedURL *url.URL) []FetchOption {
	return nil
}

// FindOrCreateFeed will either generate a new feed or return the existing feed for the given URL. If the feed is new,
// the boolean return value will be true.
func FindOrCreateFeed(ctx context.Context, feedURL string) (*models.Feed, bool, error) {
	// Fetch from URL as feed.
	newFeed, err := FetchFeed(ctx, feedURL)
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
	resp, err := elastic.Search[*models.Feed](ctx,
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

// NewFeed converts a feed source from the go-syndication library into a models.Feed object.
func NewFeed(url string, id models.FeedID, source *feeds.Feed) *models.Feed {
	if id == "" {
		id = "feed_" + strconv.FormatUint(xxh3.Hash([]byte(source.GetSourceURL())), 10)
	}
	feed := &models.Feed{
		FeedID:       id,
		CreatedAt:    time.Now().UTC(),
		LastFetched:  types.UnixEpoch,
		Title:        source.GetTitle(),
		Description:  new(source.GetDescription()),
		SourceType:   models.SourceType(source.SourceType),
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
		feed.Published = models.UnixEpoch
	}
	if updatedDate := source.GetUpdatedDate(); updatedDate != nil {
		feed.Updated = new(updatedDate.UTC())
	}

	// Extract Items from source and add to Feed. We do this in parallel as generation of some items may involve network
	// calls to fetch additional information (e.g., images).
	var wg sync.WaitGroup
	itemCh := make(chan models.Item)
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
	if !slices.Contains(feed.SourceURLs, url) {
		feed.SourceURLs = append(feed.SourceURLs, url)
	}
	// Add any image found.
	if sourceImg := source.GetImage(); sourceImg != nil {
		feed.Image = &models.RemoteImage{
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

func NewFeedSortOptions(sort *models.Sort) []estypes.SortCombinationsVariant {
	if sort == nil {
		return []estypes.SortCombinationsVariant{&estypes.SortOptions{Doc_: estypes.NewScoreSort()}}
	}
	var opts []estypes.SortCombinationsVariant
	switch *sort {
	case models.SortNewestFirst:
		opts = append(opts, &FeedSorting{
			Updated:   "desc",
			Published: "desc",
			FeedID:    "desc",
		})
	case models.SortOldestFirst:
		opts = append(opts, &FeedSorting{
			Updated:   "asc",
			Published: "asc",
			FeedID:    "asc",
		})
	case models.SortMostRelevant:
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

func NewFeedSortCombinations(sort *models.Sort) []estypes.SortCombinations {
	var opts []estypes.SortCombinations
	switch *sort {
	case models.SortNewestFirst:
		opts = append(opts, &FeedSorting{
			Updated:   "desc",
			Published: "desc",
			FeedID:    "desc",
		})
	case models.SortOldestFirst:
		opts = append(opts, &FeedSorting{
			Updated:   "asc",
			Published: "asc",
			FeedID:    "asc",
		})
	case models.SortMostRelevant:
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
