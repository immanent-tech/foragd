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
	"reflect"
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
	"golang.org/x/net/html"

	feeds "github.com/immanent-tech/go-syndication"
	"github.com/immanent-tech/go-syndication/atom"
	"github.com/immanent-tech/go-syndication/jsonfeed"
	"github.com/immanent-tech/go-syndication/opml"
	"github.com/immanent-tech/go-syndication/rdf"
	"github.com/immanent-tech/go-syndication/rss"
	"github.com/immanent-tech/go-syndication/types"

	"github.com/immanent-tech/go-base/client"
	"github.com/immanent-tech/go-base/config"
	"github.com/immanent-tech/go-base/pkg/htmlx"
	"github.com/immanent-tech/go-base/pkg/textx"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/results"
	"github.com/immanent-tech/foragd/providers/google/language"
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
	feeds := make(models.Feeds, 0, len(ids))
	unCached := make([]models.FeedID, 0)

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
		fetched, err := elastic.GetDocs[models.FeedID, *models.Feed](ctx, schema.FeedsIndexRO(), unCached...)
		if err != nil {
			return nil, fmt.Errorf("get items: %w", err)
		}
		for feed := range slices.Values(fetched) {
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

type Change struct {
	Old any `json:"old"`
	New any `json:"new"`
}

type diffReporter struct {
	path    cmp.Path
	Changes map[string]Change
}

func (r *diffReporter) PushStep(ps cmp.PathStep) {
	r.path = append(r.path, ps)
}

func (r *diffReporter) Report(rs cmp.Result) {
	if rs.Equal() {
		return
	}
	if r.Changes == nil {
		r.Changes = make(map[string]Change)
	}

	// Find the nearest StructField step to get the JSON tag.
	var tag string
	for i, v := range slices.Backward(r.path) {
		sf, ok := v.(cmp.StructField)
		if !ok {
			continue
		}
		parentType := r.path[i-1].Type()
		if parentType.Kind() == reflect.Ptr {
			parentType = parentType.Elem()
		}
		field := parentType.Field(sf.Index())
		tag = field.Tag.Get("json")
		if tag == "" || tag == "-" {
			tag = field.Name
		}
		break
	}
	if tag == "" {
		return // not inside a struct field, skip
	}

	// Grab the actual differing values from the current (leaf) step.
	last := r.path[len(r.path)-1]
	vx, vy := last.Values()

	r.Changes[tag] = Change{Old: vx.Interface(), New: vy.Interface()}
}

func (r *diffReporter) PopStep() {
	r.path = r.path[:len(r.path)-1]
}

// UpdateFeedItems determines if there are new items in the feed and then adds them to the database, performing item
// enrichment as needed. It returns a timestamp indicating a new lastFetched value for the feed, based off the latest
// new item's timestamp.
func UpdateFeedItems(ctx context.Context, oldData, newData *models.Feed) (time.Time, error) {
	// Add any new items since the last feed update.
	if len(newData.GetItems()) == 0 {
		logMsg := newFeedStatusMsg(oldData.GetID())
		logMsg.StatusCode = http.StatusNoContent
		if err := logMsg.log(ctx); err != nil {
			slogctx.Error(ctx, "Unable to write feed status.",
				slog.Any("error", err))
		}
		return oldData.LastFetched, nil
	}
	if newItems := newData.GetItems().FilterSince(oldData.LastFetched); len(newItems) > 0 {
		const maxConcurrentEnrichment = 25
		enrichJobCh := make(chan *models.Item, 100)
		var wg sync.WaitGroup
		for range maxConcurrentEnrichment {
			// Try to enrich item with additional data if possible.
			wg.Go(func() {
				for item := range enrichJobCh {
					if err := EnrichItem(ctx, oldData, item); err != nil {
						slogctx.FromCtx(ctx).Warn("Unable to enrich item.",
							slog.Any("error", err),
						)
					}
				}
			})
		}
		for item := range slices.Values(newItems) {
			enrichJobCh <- item
		}
		close(enrichJobCh)
		wg.Wait()

		// Add new items.
		results, err := AddItems(ctx, newItems)
		if err != nil {
			return oldData.LastFetched, fmt.Errorf("add new items: %w", err)
		}
		if len(results["new"]) > 0 || len(results["updated"]) > 0 {
			slogctx.FromCtx(ctx).Debug("Added new/updated items.",
				slog.Time("since", oldData.LastFetched),
				slog.Int("new", len(results["new"])),
				slog.Int("updated", len(results["updated"])),
			)
			var allItems models.Items
			for _, v := range results {
				allItems = append(allItems, v...)
			}
			logMsg := newFeedStatusMsg(oldData.GetID())
			logMsg.StatusCode = http.StatusOK
			logMsg.Items = allItems.GetIDs()
			if err := logMsg.log(ctx); err != nil {
				slogctx.Error(ctx, "Unable to write feed status.",
					slog.Any("error", err))
			}
			return allItems.SortByTimestamp()[0].GetTimestamp(), nil
		}
	}
	logMsg := newFeedStatusMsg(oldData.GetID())
	logMsg.StatusCode = http.StatusNoContent
	if err := logMsg.log(ctx); err != nil {
		slogctx.Error(ctx, "Unable to write feed status.",
			slog.Any("error", err))
	}
	return oldData.LastFetched, nil
}

// ApplyFeedUpdates takes an existing feed and new feed data, compares the two, updates any fields as appropriate and
// writes the updated feed back to the database. It will add/update both any new/updated items and any updates to the
// feed metadata.
func ApplyFeedUpdates(ctx context.Context, oldData, newData *models.Feed) error {
	// Add any new or update existing items.
	lastFetched, err := UpdateFeedItems(ctx, oldData, newData)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to add new or update existing items.",
			slog.Any("error", err))
	}
	// If the feed does not have categories, use the classifier to generate some.
	if len(oldData.GetCategories()) == 0 {
		slogctx.FromCtx(ctx).Debug("Feed needs classifying")
		// newData.Categories = service.ClassifyFeed(ctx, newData)
	} else {
		newData.Categories = oldData.Categories
	}
	// Compare new/old feed data and update as appropriate.
	var r diffReporter
	if diff := cmp.Diff(
		*oldData,
		*newData,
		cmpopts.IgnoreFields(
			models.Feed{},
			"Updated",
			"Published",
			"LastFetched",
			"CreatedAt",
			"FetchOptions",
			"SourceData",
			"Items",
			"UpdateInterval",
			"Quirks",
			"Customisation",
		),
		cmpopts.EquateEmpty(),
		cmpopts.IgnoreUnexported(),
		cmp.Reporter(&r),
	); diff != "" {
		// Update feed data.
		if oldData.LastFetched.Compare(lastFetched) < 0 {
			newData.LastFetched = lastFetched
		}
		newData.Updated = new(time.Now().UTC())
		// Re-add excluded fields that exist in the original.
		if oldData.FetchOptions != nil {
			newData.FetchOptions = oldData.FetchOptions
		}
		if oldData.SourceData != nil {
			newData.SourceData = oldData.SourceData
		}
		if oldData.Quirks != nil {
			newData.Quirks = oldData.Quirks
		}
		if oldData.Customisation != nil {
			newData.Customisation = oldData.Customisation
		}
		if err := bulk.AddAction(ctx,
			bulk.NewAction(
				newData,
				bulk.AsOperation[models.FeedID](bulk.OpIndex),
				bulk.ToIndex[models.FeedID](schema.FeedsIndexRW()),
			),
		); err != nil {
			return fmt.Errorf("update feed: %w", err)
		}

		var slogAttrs []slog.Attr
		for key, value := range r.Changes {
			slogAttrs = append(slogAttrs,
				slog.Group(key,
					slog.Any("old", fmt.Sprintf("%+v", value.Old)),
					slog.Any("new", fmt.Sprintf("%+v", value.New)),
				),
			)
		}
		slogctx.LogAttrs(ctx, slog.LevelInfo, "Feed data updated.", slogAttrs...)
	} else {
		// No changes. Just update last_fetched.
		if oldData.LastFetched.Compare(lastFetched) < 0 {
			if err := bulk.AddAction(ctx,
				bulk.NewAction(&bulk.PartialDocument{
					Parts: map[string]any{
						"last_fetched": lastFetched,
					},
					ID: newData.GetID(),
				},
					bulk.AsOperation[string](bulk.OpUpdate),
					bulk.ToIndex[string](schema.FeedsIndexRW()),
				),
			); err != nil {
				return fmt.Errorf("update feed last_fetched: %w", err)
			}
		}
	}

	return nil
}

// UpdateFeed applies the given updates to a Feed. Any cached version of the feed is invalidated.
func UpdateFeed(ctx context.Context, id models.FeedID, updates map[string]any) error {
	if err := elastic.UpdateDoc(
		ctx,
		schema.FeedsIndexRW(),
		id,
		updates,
	); err != nil {
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
			newSubscription, err := NewFeedSubscription(ctx, feed, nil)
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

// SuggestYoutubeFeeds will return a list of youtube feeds that match the given text.
func SuggestYoutubeFeeds(ctx context.Context, text string) (*models.SuggestFeedsResults, error) {
	// Get user subscriptions.
	subscriptions, err := GetAllSubscriptions(ctx)
	if err != nil && !errors.Is(err, models.ErrNotFound) {
		return nil, fmt.Errorf("get subscriptions: %w", err)
	}

	// Perform a search on youtube to find a channel that matches the user's query.
	ytResults, err := youtube.Find(ctx, text, 5)
	if err != nil {
		return nil, fmt.Errorf("search youtube channels: %w", err)
	}
	// Extract the channel RSS feed urls.
	urls := make([]string, 0, len(ytResults))
	for result := range slices.Values(ytResults) {
		urls = append(urls, result.SourceURL())
	}
	var feeds models.Feeds
	// Try to find existing feeds that match the query.
	resp, err := elastic.Search[*models.Feed](
		ctx,
		schema.FeedsIndexRO(),
		elastic.WithQueryOptions[*elastic.SearchRequest](
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
		latestItems, err := getFeedLatestItems(ctx, 3, feeds.GetIDs())
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Unable to get latest items for feeds.",
				slog.Any("error", err),
			)
		}
		return &models.SuggestFeedsResults{
			Text:        text,
			Feeds:       feeds,
			LatestItems: latestItems,
		}, nil
	}

	// Create new Youtube feeds from the results.
	feeds, err = youtube.CreateFeeds(ctx, ytResults...)
	if err != nil {
		return nil, fmt.Errorf("generate feeds: %w", err)
	}
	latestItems := make(map[models.FeedID]models.Items)
	for feed := range slices.Values(feeds) {
		latestItems[feed.GetID()] = feed.GetItems()
	}

	return &models.SuggestFeedsResults{
		Text:        text,
		Feeds:       feeds,
		LatestItems: latestItems,
	}, nil
}

// SuggestGoogleNewsFeeds will return a google news RSS feed for the given search query.
func SuggestGoogleNewsFeeds(ctx context.Context, text string) (*models.SuggestFeedsResults, error) {
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
		elastic.WithQueryOptions[*elastic.SearchRequest](
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
		latestItems, err := getFeedLatestItems(ctx, 3, feeds.GetIDs())
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Unable to get latest items for feeds.",
				slog.Any("error", err),
			)
		}
		return &models.SuggestFeedsResults{
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
	if items := newFeed.GetItems().SortByTimestamp(); len(items) > 0 {
		// Truncate to 3 items.
		if len(items) > 3 {
			items = items[:3]
		}
		latestItems[newFeed.GetID()] = items
	}
	feeds = append(feeds, newFeed)
	return &models.SuggestFeedsResults{
		Text:        text,
		Feeds:       feeds,
		LatestItems: latestItems,
	}, nil
}

// SuggestFeeds returns a feeds and their latest articles that match the given text. It will search first for existing
// feeds in Elasticsearch. If the given text is a URL, it will fallback to searching the website for a feed.
func SuggestFeeds(ctx context.Context, request *models.SuggestFeedsRequest) (*models.SuggestFeedsResults, error) {
	// Get user subscriptions.
	subscriptions, err := GetAllSubscriptions(ctx)
	if err != nil && !errors.Is(err, models.ErrNotFound) {
		return nil, fmt.Errorf("get subscriptions: %w", err)
	}

	// Find the top 5 feeds that match the user's query and which they are not already subscribed to.
	var feedSearchQuery query.Option
	switch {
	case strings.HasPrefix(request.Text, "http"):
		parsedURL, err := url.Parse(request.Text)
		if err != nil {
			return nil, fmt.Errorf("malformed URL %s: %w", request.Text, err)
		}
		feedSearchQuery = query.Bool(
			query.MustNot(
				// User must not already be subscribed.
				query.Terms("feed_id", subscriptions.GetFeedIDs()),
				// Exclude google news custom feeds.
				query.Term("domain.raw", "news.google.com"),
			),
			query.Should(
				// For URLs, match with and without a trailing slash. Boost source_URLs over URL.
				query.Term(
					"source_urls",
					strings.TrimSuffix(parsedURL.String(), "/"),
					query.WithQueryBoost[*query.TermQuery](10.0),
				),
				query.Term(
					"url",
					strings.TrimSuffix(parsedURL.String(), "/"),
					query.WithQueryBoost[*query.TermQuery](5.0),
				),
				query.Term("source_urls", parsedURL.String(), query.WithQueryBoost[*query.TermQuery](10.0)),
				query.Term("url", parsedURL.String(), query.WithQueryBoost[*query.TermQuery](5.0)),
				// Match the URL domain.
				query.Term("domain.raw", parsedURL.Hostname(), query.WithQueryBoost[*query.TermQuery](15.0)),
			),
		)
	default:
		feedSearchQuery = query.Bool(
			query.MustNot(
				// User must not already be subscribed.
				query.Terms("feed_id", subscriptions.GetFeedIDs()),
				// Exclude google news custom feeds.
				query.Term("domain.raw", "news.google.com"),
			),
			query.Must(
				// Must match any category filters.
				query.Terms("categories.raw", request.Categories),
			),
			query.Should(
				// Exact match text on title with significant boost.
				query.Term(
					"title.exact",
					request.Text,
					query.WithQueryBoost[*query.TermQuery](10.0),
				),
				// Title or description contains text, with boost for title.
				query.MultiMatch(
					request.Text,
					[]string{"title^5", "description"},
					query.WithFuzziness[*query.MultiMatchQuery]("AUTO"),
				),
				// Match phrase in description.
				query.Match("description", request.Text),
				// Match an existing subscription category.
				query.Match("categories", request.Text),
				// Try to match the domain.
				query.Match("domain", request.Text),
				query.Term("domain.raw", request.Text, query.WithQueryBoost[*query.TermQuery](15.0)),
			),
		)
	}

	suggestions := &models.SuggestFeedsResults{
		Text:        request.Text,
		Feeds:       make(models.Feeds, 0),
		LatestItems: make(map[models.FeedID]models.Items),
	}

	// Try to find existing feeds that match the query.
	resp, err := elastic.Search[*models.Feed](
		ctx,
		schema.FeedsIndexRO(),
		elastic.WithQueryOptions[*elastic.SearchRequest](feedSearchQuery),
		elastic.WithSize(request.Count),
		elastic.WithSort(NewFeedSortOptions(new(models.SortMostRelevant))...),
	)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to find feed suggestions.",
			slog.Any("error", err),
		)
	}
	if len(resp.Results) > 0 {
		// Retrieve the latest 3 articles for each feed.
		latestItems, err := getFeedLatestItems(ctx, 3, models.Feeds(resp.Results).GetIDs())
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Unable to get latest items for feeds.",
				slog.Any("error", err),
			)
		}
		suggestions.Feeds = resp.Results
		suggestions.LatestItems = latestItems
	}

	// If the request text is a URL, also
	if strings.HasPrefix(request.Text, "http") {
		if newFeedURL, err := NormalizeFeedURL(request.Text); err == nil {
			slogctx.FromCtx(ctx).Debug("Looking for new feed for URL.",
				slog.String("url", newFeedURL.String()),
			)
			newFeed, err := FetchFeed(ctx, newFeedURL.String())
			if err != nil {
				return nil, fmt.Errorf("new feed from url: %w", err)
			}
			if items := newFeed.GetItems().SortByTimestamp(); len(items) > 0 {
				// Truncate to 3 items.
				if len(items) > 3 {
					items = items[:3]
				}
				suggestions.LatestItems[newFeed.GetID()] = items
			}
			suggestions.Feeds = slices.Concat(models.Feeds{newFeed}, suggestions.Feeds)
		}
	}

	return suggestions, nil
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
	title := config.GetAppName() + " Export (" + time.Now().Format(time.DateTime) + ")"
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

// ClassifyFeed will add appropriate categories to a feed by classifying the item content.
func ClassifyFeed(ctx context.Context, feed *models.Feed) models.Categories {
	if len(feed.GetItems()) == 0 {
		return nil
	}

	ctx = slogctx.With(ctx, "feed_id", feed.GetID())

	var (
		itemText strings.Builder
	)
	// Append together all item content for classification.
	for item := range slices.Values(feed.GetItems()) {
		switch {
		case item.GetContent() != "":
			itemText.WriteString(item.GetContent())
			itemText.WriteRune('\n')
		case item.GetDescription() != "":
			itemText.WriteString(item.GetDescription())
			itemText.WriteRune('\n')
		}
	}

	// Don't classify when there is too little content for good processing.
	if textx.CountWords(itemText.String()) < 20 {
		slogctx.FromCtx(ctx).Warn("Not enough content to classify feed. Assigning 'Uncategorized'.")
		return models.Categories{"Uncategorized"}
	}

	slogctx.FromCtx(ctx).Debug("Assigning categories to feed based on current item content.",
		slog.String("feed_id", feed.GetID()))

	// Run classification.
	classifications, err := language.Classify(ctx, itemText.String())
	if err != nil {
		slogctx.Error(ctx, "Could not classify feed.",
			slog.Any("error", err))
		return nil
	}

	// Only use categories with a confidence level of at least 0.5.
	categories := make(models.Categories, 0, len(classifications))
	rejected := make(models.Categories, 0, len(classifications))
	for classification := range slices.Values(classifications) {
		if c := strings.Split(
			strings.TrimPrefix(classification.GetName(), "/"),
			"/",
		); classification.GetConfidence() > 0.5 {
			categories = append(categories, c...)
		} else {
			rejected = append(rejected, c...)
		}
	}

	// When we can't produce any good categories, just assign to "Uncategorized".
	if len(categories) == 0 {
		slogctx.FromCtx(ctx).Warn("No classified categories with high enough confidence. Assigning 'Uncategorized'.",
			slog.String("rejected_categories", strings.Join(rejected, ",")))
		categories = append(categories, "Uncategorized")
	}

	return categories
}

// DiscoverFeedURL attempts to find a feed URL within a HTML page.
//
// There are a couple of "canonical" places the feed URL is located. Firstly, as per the RSS spec, look for a link
// element with rel="alternate" and type="application/rss+xml". Secondly, check for a link element with a URL that ends
// with feed, rss or atom, which would indicate a feed URL.
//
//nolint:gocognit
func DiscoverFeedURL(sourceURL *url.URL, content []byte) (string, error) {
	page, err := html.Parse(bytes.NewReader(content))
	if err != nil {
		return "", fmt.Errorf("parse html: %w", err)
	}

	// Check if its a Medium blog with a custom domain. If so, just return the original URL with the path replaced with
	// `/feed`.
	//
	// https://help.medium.com/hc/en-us/articles/214874118-Using-RSS-feeds-of-profiles-publications-and-topics
	if htmlx.CheckMediumSignals(page) > 0 {
		sourceURL.Path = "/feed"
		return sourceURL.String(), nil
	}

	// RSS feed auto-discovery.
	//
	// https://www.rssboard.org/rss-autodiscovery
	findCanonicalLinkAttribute := func(elem *html.Node) string {
		// Needs to have attributes rel="alternate".
		if !slices.ContainsFunc(
			elem.Attr,
			func(a html.Attribute) bool { return a.Key == "rel" && a.Val == "alternate" },
		) {
			return ""
		}
		// Needs to have type="{feedMimeType}".
		if !slices.ContainsFunc(
			elem.Attr,
			func(a html.Attribute) bool { return a.Key == "type" && slices.Contains(feeds.MimeTypesFeed, a.Val) },
		) {
			return ""
		}
		// Return the href attribute value.
		for a := range slices.Values(elem.Attr) {
			if a.Key == "href" {
				return a.Val
			}
		}
		return ""
	}

	// Commonly used links based on path.
	findCommonFeedURL := func(elem *html.Node) string {
		for a := range slices.Values(elem.Attr) {
			// href attribute should contain a well-known substring.
			if a.Key == "href" &&
				(strings.HasSuffix(a.Val, "feed") || strings.Contains(a.Val, "rss") || strings.Contains(a.Val, "atom")) {
				return a.Val
			}
		}
		return ""
	}
	var findURL func(*html.Node) string
	findURL = func(n *html.Node) string {
		if n.Type == html.ElementNode && (n.Data == "a" || n.Data == "link") {
			if foundURL := findCanonicalLinkAttribute(n); foundURL != "" {
				return foundURL
			}
			if foundURL := findCommonFeedURL(n); foundURL != "" {
				return foundURL
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if foundURL := findURL(c); foundURL != "" {
				return foundURL
			}
		}
		return ""
	}

	foundURL := findURL(page)
	if foundURL == "" {
		return "", fmt.Errorf("%w: no url found", htmlx.ErrParseURL)
	}

	// Parse the discovered URL.
	feedURL, err := url.Parse(foundURL)
	if err != nil {
		return foundURL, fmt.Errorf("discover feed url: %w", err)
	}
	// Check whether the URL is absolute.
	if !feedURL.IsAbs() {
		// Try to create an absolute URL for the feed.
		feedURL, err = url.Parse(sourceURL.String())
		if err != nil {
			return "", fmt.Errorf("discover feed url: %w", err)
		}
		feedURL.Path, err = url.JoinPath("/", foundURL)
		if err != nil {
			return "", fmt.Errorf("discover feed url: %w", err)
		}
	}
	return feedURL.String(), nil
}

// NormalizeFeedURL parses the given URL string into a url.URL object, applying some additional rules for known domains on
// where to find their feeds.
func NormalizeFeedURL(urlStr string) (*url.URL, error) {
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

// getFeedLatestItems fetches the most recent count items for each given feed. An optional query clause can be specified
// that will be added to the bool filter clause of the query to apply additional filtering to the items.
func getFeedLatestItems(
	ctx context.Context,
	count int,
	feedIDs []models.FeedID,
) (map[models.FeedID]models.Items, error) {
	resp, err := elastic.Search[*models.Item](ctx,
		schema.ItemsIndexRO(),
		elastic.WithQueryOptions[*elastic.SearchRequest](
			query.Bool(
				query.Filter(
					query.Terms("feed_id", feedIDs),
				),
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
				if feedID == "" {
					return
				}
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

func getFeedLastUpdates(ctx context.Context, ids ...models.FeedID) (map[models.FeedID]time.Time, error) {
	sort := models.SortNewestFirst
	resp, err := elastic.Search[*models.Item](
		ctx,
		schema.ItemsIndexRO(),
		elastic.WithQueryOptions[*elastic.SearchRequest](query.Terms("feed_id", ids)),
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
		elastic.WithQueryOptions[*elastic.SearchRequest](query),
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
	sourceURL, err := url.Parse(feedURL)
	if err != nil {
		return nil, models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("parse url: %w", err))
	}

	// Create a buffer for the feed data.
	feedBuf, ok := bufPool.Get().(*bytes.Buffer)
	if !ok {
		return nil, models.NewAPIError(http.StatusInternalServerError, errors.New("get feed buffer failed"))
	}
	feedBuf.Reset()
	defer bufPool.Put(feedBuf)

	// Fetch the feed data from the source URL.
	var contentType string
	switch opts.Proxy {
	case false:
		slogctx.FromCtx(ctx).Debug("Fetching feed directly.",
			slog.String("feed_url", sourceURL.String()),
		)
		client, err := client.Load()
		if err != nil {
			return nil, fmt.Errorf("load http client: %w", err)
		}

		resp, err := client.R().
			SetContext(ctx).
			SetHeader("User-Agent", config.GetAppName()+"/"+config.GetVersion()+" (+https://foragd.app/policies/bot)").
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

	data, err := io.ReadAll(feedBuf)
	if err != nil {
		return nil, fmt.Errorf("read feed data: %w", err)
	}

	// Parse the response as a feed type.
	var feedData *feeds.Feed
	switch feedType, err := feeds.DetectSourceType(bytes.NewReader(data)); {
	case err != nil:
		return nil, fmt.Errorf("detect feed type: %w", err)
	case feedType == types.SourceUnknown:
		return nil, models.NewAPIError(
			http.StatusUnsupportedMediaType,
			errors.New("cannot determine feed type"),
		)
	case feedType == types.SourceAtom:
		// Atom feed.
		feedData, err = feeds.NewDecoder[*atom.Feed](bytes.NewReader(data))
		if err != nil {
			return nil, models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("parse atom: %w", err))
		}
	case feedType == types.SourceRSS:
		// RSS 2.0 feed.
		feedData, err = feeds.NewDecoder[*rss.RSS](bytes.NewReader(data))
		if err != nil {
			return nil, models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("parse rss: %w", err))
		}
	case feedType == types.SourceRDF:
		// RDF/RSS 1.0 feed.
		feedData, err = feeds.NewDecoder[*rdf.RDF](bytes.NewReader(data))
		if err != nil {
			return nil, models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("parse rss: %w", err))
		}
	case feedType == types.SourceJSONFeed:
		// JSONFeed.
		feedData, err = feeds.NewDecoder[*jsonfeed.Feed](bytes.NewReader(data))
		if err != nil {
			return nil, models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("parse rss: %w", err))
		}
	case feedType == types.SourceHTML:
		// HTML web page. Use "autodiscovery" to find feed.
		if newURL, err := DiscoverFeedURL(
			sourceURL,
			data,
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

	feed := models.NewFeed(feedData.GetSourceURL(), opts.FeedID, feedData)

	// Set the feed update interval to an appropriate value.
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

	// Set the method used to fetch the feed.
	if opts.Proxy {
		feed.FetchMethod = models.FeedFetchMethodProxied
	} else {
		feed.FetchMethod = models.FeedFetchMethodDirect
	}

	return feed, nil
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
		elastic.WithQueryOptions[*elastic.SearchRequest](
			query.Bool(
				query.Filter(
					query.Bool(
						query.Should(terms...),
					),
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

// FetchFeedUpdates fetches an updated version of the feed, including items. It returns the updated feed, and the source
// URL used to fetch the updates (for disambiguation of feeds with multiple source URLs). A non-nil error is returned
// where there is a critical error fetching the feed details.
func FetchFeedUpdates(ctx context.Context, details *models.Feed) (*models.Feed, models.URL, error) {
	// Set fetch options.
	var (
		proxyRequest bool
	)
	if details.FetchMethod == models.FeedFetchMethodProxied {
		proxyRequest = true
	}

	// Get new items since the last fetch. Try each listed source URL for the feed until one succeeds.
	var errs []error
	for feedURL := range slices.Values(details.GetSourceURLs()) {
		feed, err := FetchFeed(
			ctx,
			feedURL,
			FetchWithFeedID(details.GetID()),
			FetchWithProxy(proxyRequest),
		)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", feedURL, err))
			logGeneralError(ctx, err, feedURL, details.GetID())
			continue
		}
		return feed, feedURL, nil
	}

	// No feed data returned by any url. Log and return error.
	err := models.NewAPIError(
		http.StatusNoContent,
		errors.New("failed to fetch feed details with any source URL: "+errors.Join(errs...).Error()),
	)
	logGeneralError(ctx, err, strings.Join(details.GetSourceURLs(), ","), details.GetID())
	return nil, "", err
}

// FetchFeedUpdatesAsArticles fetches an updated version of the feed, using Zyte to generate a feed from the source
// website articles list. It returns the updated feed, and the source URL used to fetch the updates (for disambiguation
// of feeds with multiple source URLs). A non-nil error is returned where there is a critical error fetching the feed
// details.
func FetchFeedUpdatesAsArticles(ctx context.Context, details *models.Feed) (*models.Feed, models.URL, error) {
	// Get any extraction options from the feed.
	var extractOptions zyte.ExtractOptions
	if details.FetchOptions != nil {
		var err error
		extractOptions, err = details.FetchOptions.AsFetchZyteOptions()
		if err != nil {
			return nil, "", fmt.Errorf("extract fetch options: %w", err)
		}
	}

	// Set from where the list will be extracted.
	extractFrom := zyte.ExtractFromHttpResponseBody
	if extractOptions.ExtractFrom != nil {
		extractFrom = *extractOptions.ExtractFrom
	}

	// Get new items since the last fetch. Try each listed source URL for the feed until one succeeds.
	var errs []error
	for feedURL := range slices.Values(details.GetSourceURLs()) {
		// Fetch the feed details using Zyte as an article list.
		resp, err := zyte.Proxy(
			ctx,
			feedURL,
			zyte.WithExtractFrom(extractFrom),
			zyte.AsArticleList(&extractOptions),
			zyte.WithTag("feed_id", details.GetID()),
			zyte.WithTag("action", "update_feed"),
		)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", feedURL, err))
			logZyteError(ctx, err, feedURL, details.GetID())
			continue
		}
		// Generate feed details from Zyte response.
		feed, err := NewFeedFromZyteResponse(ctx, resp)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", feedURL, err))
			logGeneralError(ctx, err, feedURL, details.GetID())
			continue
		}

		// Extract and enrich articles from Zyte response.
		items, err := NewItemsFromZyteArticles(ctx, feed, resp.ArticleList)
		if err != nil {
			logGeneralError(ctx, err, feedURL, details.GetID())
			return nil, feedURL, fmt.Errorf("%s: %w", feedURL, err)
		}
		feed.Items = items

		return feed, feedURL, nil
	}

	// No feed data returned by any url. Log and return error.
	err := models.NewAPIError(
		http.StatusNoContent,
		errors.New("failed to fetch feed details with any source URL: "+errors.Join(errs...).Error()),
	)
	logGeneralError(ctx, err, strings.Join(details.GetSourceURLs(), ","), details.GetID())
	return nil, "", err
}

func NewFeedFromZyteResponse(ctx context.Context, resp *zyte.Response) (*models.Feed, error) {
	// Create a buffer for the feed data.
	feedBuf, ok := bufPool.Get().(*bytes.Buffer)
	if !ok {
		return nil, models.NewAPIError(http.StatusInternalServerError, errors.New("get feed buffer failed"))
	}
	feedBuf.Reset()
	defer bufPool.Put(feedBuf)
	// Extract the response body
	body, err := resp.GetBody()
	if err != nil {
		return nil, models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("get response body: %w", err))
	}
	if _, err := feedBuf.Write(body); err != nil {
		return nil, models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("read response: %w", err))
	}

	sourceURL, err := url.Parse(resp.URL)
	if err != nil {
		return nil, models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("parse response URL: %w", err))
	}

	feedID := "feed_" + strconv.FormatUint(xxh3.Hash([]byte(sourceURL.String())), 10)

	// Extract metadata (opengraph, readability) from HTML.
	ogData, rdData, err := extractMetadataFromHTML(sourceURL, body)
	if err != nil {
		logGeneralError(ctx, err, resp.URL, feedID)
	}

	feed := &models.Feed{
		FeedID:      feedID,
		CreatedAt:   time.Now().UTC(),
		LastFetched: models.UnixEpoch,
		SourceType:  models.SourceTypeRSS,
		SourceURLs:  []string{sourceURL.String()},
		URL:         sourceURL.String(),
		Domain:      sourceURL.Hostname(),
	}
	if resp.ArticleList != nil {
		feed.FetchMethod = models.FeedFetchMethodZyteArticles
	}

	// Parse info from opengraph data.
	if ogData != nil {
		if ogData.Title != "" {
			feed.Title = ogData.Title
		}
		if ogData.Description != "" {
			feed.Description = &ogData.Description
		}
		if ogData.Image != "" {
			feed.Image = models.NewRemoteImage(ogData.Image, feed.Title)
		}
	}
	// Parse info from readability data.
	if rdData != nil {
		if rdData.Title() != "" && feed.Title != "" {
			feed.Title = rdData.Title()
		}
		if rdData.Byline() != "" && feed.Description == nil {
			feed.Description = new(rdData.Byline())
		}
	}

	// Parse timestamps.
	if published, err := rdData.PublishedTime(); err != nil {
		feed.Published = models.UnixEpoch
	} else {
		feed.Published = published.UTC()
	}
	if updated, _ := rdData.ModifiedTime(); !updated.IsZero() {
		updUTC := updated.UTC()
		feed.Updated = &updUTC
	}

	// Set extract options
	var extractFrom zyte.ExtractFrom
	switch {
	case resp.BrowserHtml != nil:
		extractFrom = zyte.ExtractFromBrowserHtml
	case resp.HttpResponseBody != nil:
		fallthrough
	default:
		extractFrom = zyte.ExtractFromHttpResponseBody
	}
	var fetchOptions models.Feed_FetchOptions
	if err := fetchOptions.FromFetchZyteOptions(models.FetchZyteOptions{ExtractFrom: &extractFrom}); err != nil {
		return feed, models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("set fetch options: %w", err))
	}
	feed.FetchOptions = &fetchOptions

	// Make sure we have generated valid feed data.
	if err := feed.Validate(); err != nil {
		return feed, models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("validate feed: %w", err))
	}

	return feed, nil
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

type feedStatusLogMsg struct {
	*models.FeedStatus

	Labels map[string]string `json:"labels"`
}

func newFeedStatusMsg(id models.FeedID) *feedStatusLogMsg {
	return &feedStatusLogMsg{
		FeedStatus: &models.FeedStatus{
			Timestamp: time.Now().UTC(),
			FeedID:    id,
		},
		Labels: map[string]string{
			"env":  config.GetEnvironment().String(),
			"type": "feed-status",
		},
	}
}

func (l *feedStatusLogMsg) log(ctx context.Context) error {
	if err := bulk.AddAction(ctx,
		bulk.NewAction(
			l,
			bulk.AsOperation[string](bulk.OpIndex),
			bulk.ToIndex[string]("logs"),
		),
	); err != nil {
		return fmt.Errorf("add bulk action: %w", err)
	}
	return nil
}

func logZyteError(ctx context.Context, err error, feedURL string, feedID models.FeedID) {
	logMsg := newFeedStatusMsg(feedID)
	logMsg.FeedStatus.URL = feedURL
	if apiErr, ok := errors.AsType[*models.APIError](err); ok {
		logMsg.StatusCode = apiErr.StatusCode
		logMsg.StatusMessage = new(apiErr.Error())
	} else {
		logMsg.StatusCode = http.StatusInternalServerError
		logMsg.StatusMessage = new(err.Error())
	}
	if err := logMsg.log(ctx); err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to record feed status.",
			slog.Any("error", err),
		)
	}
}

func logGeneralError(ctx context.Context, err error, feedURL string, feedID models.FeedID) {
	logMsg := newFeedStatusMsg(feedID)
	logMsg.FeedStatus.URL = feedURL
	if apiErr, ok := errors.AsType[*models.APIError](err); ok {
		logMsg.StatusCode = apiErr.StatusCode
		logMsg.StatusMessage = new(apiErr.Error())
	} else {
		logMsg.StatusCode = http.StatusInternalServerError
		logMsg.StatusMessage = new(err.Error())
	}
	if err := logMsg.log(ctx); err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to record feed status.",
			slog.Any("error", err),
		)
	}
}
