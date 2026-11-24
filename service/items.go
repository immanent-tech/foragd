// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"runtime"
	"slices"
	"strconv"
	"sync"
	"time"

	"codeberg.org/readeck/go-readability/v2"
	"github.com/go-resty/resty/v2"
	"github.com/maypok86/otter/v2"
	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"
	slogctx "github.com/veqryn/slog-context"
	"github.com/zeebo/xxh3"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/go-base/pkg/htmlx"
	"github.com/immanent-tech/go-base/validation"

	"github.com/immanent-tech/go-base/pkg/zyte"

	"github.com/immanent-tech/foragd/models"
	gerror "github.com/immanent-tech/foragd/providers/google/error"
	"github.com/immanent-tech/foragd/providers/opensearch"
	"github.com/immanent-tech/foragd/providers/opensearch/bulk"
	"github.com/immanent-tech/foragd/providers/opensearch/doc"
	"github.com/immanent-tech/foragd/providers/opensearch/search"
	"github.com/immanent-tech/foragd/schema"
	"github.com/immanent-tech/foragd/server/cache"
)

const dateRangeRounding = "||/m"

// itemCacheExpiryCalculator is a custom expiry calculator for the feed cache.
type itemCacheexpiryCalculator struct{}

// ExpireAfterCreate sets expiration time for new entries.
func (ec *itemCacheexpiryCalculator) ExpireAfterCreate(_ otter.Entry[models.ItemID, *models.Item]) time.Duration {
	return 24 * time.Hour
}

// ExpireAfterUpdate sets expiration time after updates.
func (ec *itemCacheexpiryCalculator) ExpireAfterUpdate(
	_ otter.Entry[models.ItemID, *models.Item],
	_ *models.Item,
) time.Duration {
	return 24 * time.Hour
}

// ExpireAfterRead returns remaining expiration time for reads.
func (ec *itemCacheexpiryCalculator) ExpireAfterRead(entry otter.Entry[models.ItemID, *models.Item]) time.Duration {
	return 24 * time.Hour // Extend by 24 hours.
}

// ItemService holds a cache and backend connection for handling [models.Item] objects.
type ItemService struct {
	*otter.Cache[models.ItemID, *models.Item]

	bulkLoader otter.BulkLoaderFunc[models.ItemID, *models.Item]
	osc        *opensearch.Client
	indexer    *bulk.Indexer
	dataCache  cache.ObjectCache
	httpClient *resty.Client

	closer closeOnce
}

// LoadItemsService loads a service that can manipulate [models.Item] objects in the backend store.
func LoadItemsService(
	ctx context.Context,
	osc *opensearch.Client,
	indexer *bulk.Indexer,
	httpClient *resty.Client,
) (*ItemService, error) {
	if osc == nil {
		return nil, errors.New("no opensearch client")
	}
	if indexer == nil {
		return nil, errors.New("no indexer")
	}
	if httpClient == nil {
		return nil, errors.New("no http client")
	}

	itemsCache, err := cache.NewItemsCache()
	if err != nil {
		return nil, fmt.Errorf("load items cache: %w", err)
	}

	return &ItemService{
		Cache: otter.Must(&otter.Options[models.ItemID, *models.Item]{
			MaximumSize:      10_000,
			ExpiryCalculator: &itemCacheexpiryCalculator{},
		}),
		bulkLoader: otter.BulkLoaderFunc[models.ItemID, *models.Item](
			func(ctx context.Context, keys []models.ItemID) (map[models.ItemID]*models.Item, error) {
				items, err := doc.GetDocs[models.ItemID, models.Item](
					ctx,
					osc.Doc,
					osc.IndexReadAlias(schema.ItemsIndex),
					keys...)
				if err != nil {
					return nil, fmt.Errorf("mget items: %w", err)
				}
				itemMap := make(map[models.ItemID]*models.Item)
				for item := range slices.Values(items) {
					itemMap[item.GetID()] = &item
				}
				return itemMap, nil
			},
		),
		dataCache:  itemsCache,
		osc:        osc,
		indexer:    indexer,
		httpClient: httpClient,
	}, nil
}

func (s *ItemService) Close() error {
	// The shutdown context must not derive from the (already cancelled) signal context.
	shutdownCtx, cancel := context.WithTimeoutCause(
		context.Background(),
		20*time.Second,
		errors.New("graceful shutdown timeout"),
	)
	shutdownCtx = slogctx.NewCtx(shutdownCtx, slog.Default())
	defer cancel()

	slogctx.Info(shutdownCtx, "Shutting down items service")

	return s.closer.do(func() error {
		s.Cache.CleanUp()
		return nil
	})
}

// GetItems returns a [models.Items] slice containing the items matching the given [models.ItemID].
func (s *ItemService) GetItems(ctx context.Context, ids ...models.ItemID) (models.Items, error) {
	items, err := s.BulkGet(ctx, ids, s.bulkLoader)
	if err != nil {
		return nil, fmt.Errorf("bulk get items: %w", err)
	}
	if len(items) > len(ids) {
		slogctx.Warn(ctx, "Fetched items is less than requested items.")
	}
	// Otter's bulk loader can return values that are nil. Filter those out.
	cleaned := make(map[models.ItemID]*models.Item, len(items))
	for k, v := range items {
		if v != nil {
			cleaned[k] = v
		}
	}
	return slices.Collect(maps.Values(cleaned)), nil
}

// AddItems will add the given [models.Item] to the database. It returns a map divided into "updated" and "new" items,
// to indicate items that existed and were updated vs. items that were added as new.
func (s *ItemService) AddItems(ctx context.Context, items models.Items) (int, error) {
	for item := range slices.Values(items) {
		if err := s.indexer.Add(ctx, bulk.NewAction(
			bulk.ActionCreate,
			bulk.ToIndex(s.osc.IndexWriteAlias(schema.ItemsIndex)),
			bulk.WithID(item.GetID()),
			bulk.WithDoc(item),
		)); err != nil {
			slogctx.Warn(
				ctx,
				"Could not bulk add item",
				slog.String("item_id", item.GetID()),
				slog.Any("error", err),
			)
			continue
		}
	}
	if err := s.indexer.Flush(ctx); err != nil {
		slogctx.Warn(ctx, "Flush bulk items failed", slog.Any("error", err))
	}
	if err := ctx.Err(); err != nil {
		slogctx.Warn(ctx, "context done",
			slog.Any("cause", context.Cause(ctx)),
			slog.Any("error", err),
		)
		return 0, fmt.Errorf("add items cancelled: %w", err)
	}
	// Update cache.
	for item := range slices.Values(items) {
		s.Set(item.GetID(), item)
	}

	return len(items), nil
}

type ItemChanges struct {
	New       models.Items
	Updated   models.Items
	Unchanged models.Items
}

// SortItemsByState takes a [models.Items] slice and checks for significant changes for the existing items in the
// slice. It returns an [ItemChanges] which contains slices of the new, updated, and unchanged items.
func (s *ItemService) SortItemsByState(ctx context.Context, items models.Items) ItemChanges {
	oldItems, err := s.GetItems(ctx, items.GetIDs()...)
	if err != nil {
		// If we can't fetch existing items, assume all items are new so enrichment takes place.
		slogctx.Warn(ctx, "Could not fetch existing items for filtering. Treating all items as new.",
			slog.Any("error", err))
		return ItemChanges{
			New: items,
		}
	}
	oldItemsMap := oldItems.AsMap()
	newItems := make(models.Items, 0)
	updatedItems := make(models.Items, 0)
	unchangedItems := make(models.Items, 0)
	for item := range slices.Values(items) {
		oldItem, found := oldItemsMap[item.GetID()]
		if !found {
			// Item is new.
			newItems = append(newItems, item)
		} else {
			// If the title, description, or content has changed, consider the item changed.
			if oldItem.GetTitle() != item.GetTitle() || oldItem.GetDescription() != item.GetDescription() ||
				oldItem.GetContent() != item.GetContent() {
				updatedItems = append(updatedItems, item)
			} else {
				// Items that are not new and don't have significant content changes are discarded.
				unchangedItems = append(unchangedItems, item)
			}
		}
	}
	return ItemChanges{New: newItems, Updated: updatedItems, Unchanged: unchangedItems}
}

// EnrichItemBatch performs item enrichment on the given slice of items for the given feed. Individual item enrichments
// failures are non-fatal, logged and do not halt processing any remaining items. EnrichItemBatch passes all processed
// items to AddItems and returns the result values of that method.
func (s *ItemService) EnrichItemBatch(
	ctx context.Context,
	feed *models.Feed,
	items models.Items,
) (int, error) {
	// Determine bounded concurrency
	maxWorkers := min(runtime.GOMAXPROCS(0)*4, 100)

	var (
		g  errgroup.Group
		mu sync.Mutex
	)
	g.SetLimit(maxWorkers)
	errs := make([]error, 0)
	results := make(chan *models.Item, len(items))

	// Fan-out: send items to workers
	for item := range slices.Values(items) {
		g.Go(func() error {
			if err := s.EnrichItem(ctx, feed, item); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
			results <- item
			return nil
		})
	}

	_ = g.Wait()
	if len(errs) > 0 {
		slogctx.Warn(
			ctx,
			"Some items failed to be enriched.",
			slog.Int("count", int(len(errs))),
			slog.Any("errors", errors.Join(errs...)),
		)
	}

	close(results)

	// Fan-in: collect results
	var enrichedItems models.Items
	for item := range results {
		enrichedItems = append(enrichedItems, item)
	}

	// Add/update items in bulk
	return s.AddItems(ctx, enrichedItems)
}

// EnrichItem checks the data of the given [models.Item]. If it is missing certain values, it tries to "enrich" the item
// to fill missing data from the item source.
func (s *ItemService) EnrichItem(
	ctx context.Context,
	feed *models.Feed,
	item *models.Item,
) error {
	if err := ctx.Err(); err != nil {
		slogctx.Warn(ctx, "Cannot enrich item: context done",
			slog.Any("cause", context.Cause(ctx)),
			slog.Any("error", err),
		)
		return fmt.Errorf("enrich item cancelled: %w", err)
	}

	defer func() {
		if rvr := recover(); rvr != nil {
			slogctx.Error(ctx, "Item enrichment failed!",
				slog.String("item_id", item.GetID()),
			)
			// Log to GCP error console.
			switch v := rvr.(type) {
			case error:
				gerror.ReportError(v)
			default:
				gerror.ReportError(fmt.Errorf("panic: %v", v))
			}
		}
	}()

	itemCtx, cancel := context.WithTimeoutCause(ctx, 5*time.Minute, errors.New("enrich item timeout"))
	defer cancel()

	itemCtx = slogctx.With(itemCtx,
		slog.String("feed_id", item.GetFeedID()),
		slog.String("item_id", item.GetID()),
		slog.String("url", item.GetLink()),
	)

	itemURL, err := url.Parse(item.GetLink())
	if err != nil {
		return models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("parse item link: %w", err))
	}
	if !itemURL.IsAbs() {
		return models.NewAPIError(
			http.StatusUnprocessableEntity,
			fmt.Errorf("%q is not an absolute URL", itemURL.String()),
		)
	}

	var fetchSummaries bool

	// Check if the feed indicates summaries should be fetched separately.
	if feed.FetchMethod == models.FeedFetchMethodDirect || feed.FetchMethod == models.FeedFetchMethodProxied {
		if feed.FetchOptions != nil {
			if fetchOptions, err := feed.FetchOptions.AsFetchDirectOptions(); err != nil {
				slogctx.Warn(itemCtx, "Unable to parse feed fetch options.",
					slog.Any("error", err))
			} else {
				if fetchOptions.FetchItemSummaries {
					fetchSummaries = true
				}
			}
		}
	}
	// Check if the item has a summary.
	if item.GetDescription() == "" {
		fetchSummaries = true
	}

	// Flag if the item needs enrichment.
	var needsEnriching bool
	if fetchSummaries {
		needsEnriching = true
	}
	if item.GetImage() == nil {
		needsEnriching = true
	}
	// Bail if no enrichment needs to be done.
	if !needsEnriching {
		return nil
	}

	// Get the item content, either from the cache or fetch fresh.
	itemContentBuf, err := s.getItemContent(itemCtx, item.GetID(), itemURL)
	if err != nil {
		return models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("get item content: %w", err))
	}

	// Extract opengraph and readability data from item HTML source.
	opengraphData, readabilityData, err := extractMetadataFromHTML(itemURL, itemContentBuf.Bytes())
	if err != nil {
		logGeneralError(itemCtx, s.indexer, err, feed.GetSourceURLs()[0], feed.GetID())
	}

	// Add an image if needed.
	if item.GetImage() == nil {
		item.Image = enrichItemImage(item, itemURL, itemContentBuf, opengraphData, readabilityData)
	}

	// When item summaries needed to be fetched, check and add an item description if missing.
	if fetchSummaries {
		switch {
		case opengraphData != nil && opengraphData.Description != "":
			item.Description = &opengraphData.Description
		case readabilityData != nil && readabilityData.Excerpt() != "":
			desc := readabilityData.Excerpt()
			item.Description = &desc
		}
	}

	return nil
}

func enrichItemImage(
	item *models.Item,
	itemURL *url.URL,
	contentBuf *bytes.Buffer,
	opengraphData *htmlx.OpenGraph,
	readabilityData *readability.Article,
) *models.RemoteImage {
	switch {
	case opengraphData != nil && opengraphData.Image != "":
		if imgURL, err := url.Parse(opengraphData.Image); err == nil {
			if !imgURL.IsAbs() {
				return models.NewRemoteImage(itemURL.ResolveReference(imgURL).String(), opengraphData.Title)
			} else {
				return models.NewRemoteImage(imgURL.String(), opengraphData.Title)
			}
		}
	case readabilityData != nil && readabilityData.ImageURL() != "":
		if imgURL, err := url.Parse(readabilityData.ImageURL()); err == nil {
			if !imgURL.IsAbs() {
				return models.NewRemoteImage(itemURL.ResolveReference(imgURL).String(), readabilityData.Title())
			} else {
				return models.NewRemoteImage(imgURL.String(), readabilityData.Title())
			}
		}
	default:
		if imgURL, imgAlt, _ := htmlx.ExtractImage(contentBuf.String(), item.GetLink()); imgURL != "" {
			return models.NewRemoteImage(imgURL, imgAlt)
		}
	}
	return nil
}

func (s *ItemService) GetTopCategoriesForItems(
	ctx context.Context,
	itemsQuery search.QueryOption,
) (models.CategoryCounts, error) {
	resp, err := search.Search(ctx, s.osc, s.osc.IndexReadAlias(schema.ItemsIndex),
		search.WithQuery(itemsQuery),
		search.WithAggregation("CategoryCounts",
			search.TermsAggregation(
				search.WithTermsAggregationField("categories.raw"),
				search.WithTermsAggregationSize(20),
			),
		),
		search.WithSize(0),
		search.WithTrackTotalHits(false),
		search.WithTrackScores(false),
		search.WithSort(search.NewSortOptions(search.WithSortField("_doc"))),
	)
	if err != nil {
		return nil, fmt.Errorf("search items: %w", err)
	}
	categoryCounts, err := resp.ExtractAggregation[opensearchapi.CommonAggregationsStringTermsAggregate](
		"CategoryCounts",
	)
	if err != nil {
		return nil, fmt.Errorf("extract aggregation: %w", err)
	}

	// Loop through the aggregation results and extract the unread count for each feed.
	counts := make(models.CategoryCounts, 0, len(categoryCounts.Buckets))
	for bucket := range slices.Values(categoryCounts.Buckets) {
		if bucket.Key == "" {
			continue
		}
		counts = append(counts, models.CategoryCount{Category: bucket.Key, Count: int(bucket.DocCount)})
	}
	return counts, nil
}

func (s *ItemService) GetTopLanguageCountsForItems(
	ctx context.Context,
	itemsQuery search.QueryOption,
) (map[string]int64, error) {
	resp, err := search.Search(ctx, s.osc, s.osc.IndexReadAlias(schema.ItemsIndex),
		search.WithQuery(itemsQuery),
		search.WithAggregation("LanguageCounts",
			search.TermsAggregation(
				search.WithTermsAggregationField("language"),
				search.WithTermsAggregationSize(20),
			),
		),
		search.WithSize(0),
		search.WithTrackTotalHits(false),
		search.WithTrackScores(false),
		search.WithSort(search.NewSortOptions(search.WithSortField("_doc"))),
	)
	if err != nil {
		return nil, fmt.Errorf("search items: %w", err)
	}
	languageCounts, err := resp.ExtractAggregation[opensearchapi.CommonAggregationsStringTermsAggregate](
		"LanguageCounts",
	)
	if err != nil {
		return nil, fmt.Errorf("extract aggregation: %w", err)
	}

	// Loop through the aggregation results and extract the unread count for each feed.
	counts := make(map[string]int64, len(languageCounts.Buckets))
	for bucket := range slices.Values(languageCounts.Buckets) {
		if bucket.Key == "" {
			continue
		}
		counts[bucket.Key] = bucket.DocCount
	}
	return counts, nil
}

// BuildItemQueries generates a slices of queries for the given subscriptions, based on the given filters.
func BuildItemQueries(
	user *models.User,
	view models.View,
	subscriptions models.Subscriptions,
) []search.QueryOption {
	queries := make([]search.QueryOption, 0, len(subscriptions))

	// Ignore if no subscriptions specified.
	if len(subscriptions) == 0 {
		return nil
	}

	// Filter to favorites.
	if view == models.ViewFavorites {
		return []search.QueryOption{
			search.TermsQuery("item_id", user.ItemFavorites),
		}
	}

	// Filter based on read/unread/all.
	grouped := make(map[time.Time][]models.FeedID)
	filtered := make(models.Subscriptions, 0)
	unreadItems := make([]models.ItemID, 0)
	readItems := make([]models.ItemID, 0)
	for subscription := range slices.Values(subscriptions) {
		// Ignore subscriptions that aren't based on a feed object.
		if subscription.GetFeedID() == "" {
			continue
		}
		// Gather read and unread item IDs.
		unreadItems = append(unreadItems, subscription.GetUnreadItems()...)
		readItems = append(readItems, subscription.GetReadItems()...)
		if filters := subscription.GetArticleFilters(); filters != nil && !filters.IsEmpty() {
			// Record subscriptions with article filters.
			filtered = append(filtered, subscription)
		} else {
			// Group subscriptions by the 5m window in which they were marked read.
			tsIdx := roundDownTo5Min(subscription.GetMarkedReadAt())
			feedIDs := grouped[tsIdx]
			feedIDs = append(feedIDs, subscription.GetFeedID())
			grouped[tsIdx] = feedIDs
		}
	}

	switch view {
	case models.ViewAll:
		shouldClauses := allSubscriptionFilters(grouped, filtered, user.GetMaxHistory())
		// Must match any subscription clause.
		viewQuery := search.BoolQuery(
			search.WithBoolShould(shouldClauses...),
		)
		queries = append(queries, viewQuery)
	case models.ViewRead:
		shouldClauses := readSubscriptionFilters(grouped, filtered, user.GetMaxHistory())
		shouldClauses = append(shouldClauses, search.TermsQuery("item_id", readItems))
		readQuery := search.BoolQuery(
			search.WithBoolMustNot(
				search.TermsQuery("item_id", unreadItems),
			),
			search.WithBoolShould(shouldClauses...),
		)
		queries = append(queries, readQuery)
	case models.ViewUnread:
		fallthrough
	default:
		shouldClauses := unReadSubscriptionFilters(grouped, filtered, user.GetMaxHistory())
		shouldClauses = append(shouldClauses, search.TermsQuery("item_id", unreadItems))
		unreadQuery := search.BoolQuery(
			search.WithBoolMustNot(
				search.TermsQuery("item_id", readItems),
			),
			search.WithBoolShould(shouldClauses...),
		)
		queries = append(queries, unreadQuery)
	}

	return queries
}

// unReadSubscriptionFilters generates a set of query options to fetch unread articles for the given subscriptions.
func unReadSubscriptionFilters(
	grouped map[time.Time][]models.FeedID,
	filtered models.Subscriptions,
	maxHistory time.Time,
) []search.QueryOption {
	filters := make([]search.QueryOption, 0)
	// Add filters for grouped subscriptions.
	for ts, feedIDs := range grouped {
		since := mostRecent(ts, maxHistory).Format("2006-01-02T15:04:05Z")
		filters = append(filters,
			search.BoolQuery(
				search.WithBoolFilter(
					search.TermsQuery("feed_id", feedIDs),
					search.BoolQuery(
						search.WithBoolMinimumShouldMatch(1),
						search.WithBoolShould(
							search.SinceQuery("published", since),
							search.SinceQuery("updated", since),
						),
					),
				),
			),
		)
	}
	// Add filters for subscriptions with article filters.
	for subscription := range slices.Values(filtered) {
		filters = append(filters, UnreadItemsForSubscriptionClause(subscription, maxHistory))
	}
	return filters
}

// UnreadItemsForSubscriptionClause generates a bool query that can be used to get unread items for a subscription.
func UnreadItemsForSubscriptionClause(subscription *models.Subscription, maxHistory time.Time) search.QueryOption {
	since := mostRecent(subscription.GetMarkedReadAt(), maxHistory).Format("2006-01-02T15:04:05Z")
	return search.BoolQuery(
		search.WithBoolFilter(
			search.TermQuery("feed_id", subscription.GetFeedID()),
			search.BoolQuery(
				search.WithBoolMinimumShouldMatch(1),
				search.WithBoolShould(
					search.SinceQuery("published", since),
					search.SinceQuery("updated", since),
				),
			),
		),
		search.WithBoolMustNot(
			search.TermsQuery("item_id", subscription.GetReadItems()),
		),
		search.WithBoolShould(
			search.TermsQuery("item_id", subscription.GetUnreadItems()),
		),
		ArticleFiltersQueryClause(subscription.GetArticleFilters()),
	)
}

// readSubscriptionFilters generates a set of query options to fetch read articles for the given subscriptions.
func readSubscriptionFilters(
	grouped map[time.Time][]models.FeedID,
	filtered models.Subscriptions,
	maxHistory time.Time,
) []search.QueryOption {
	filters := make([]search.QueryOption, 0)
	// Add filters for grouped subscriptions.
	for ts, feedIDs := range grouped {
		before := mostRecent(ts, maxHistory).Format("2006-01-02T15:04:05Z")
		filters = append(filters,
			search.BoolQuery(
				search.WithBoolFilter(
					search.TermsQuery("feed_id", feedIDs),
					search.BoolQuery(
						search.WithBoolMinimumShouldMatch(1),
						search.WithBoolShould(
							search.BeforeQuery("published", before),
							search.BeforeQuery("updated", before),
						),
					),
				),
			),
		)
	}
	// Add filters for subscriptions with article filters.
	for subscription := range slices.Values(filtered) {
		filters = append(filters, ReadItemsForSubscriptionClause(subscription, maxHistory))
	}
	return filters
}

// ReadItemsForSubscriptionClause generates a bool query that can be used to get read items for a subscription.
func ReadItemsForSubscriptionClause(subscription *models.Subscription, maxHistory time.Time) search.QueryOption {
	before := mostRecent(subscription.GetMarkedReadAt(), maxHistory).Format("2006-01-02T15:04:05Z")
	return search.BoolQuery(
		search.WithBoolFilter(
			search.TermQuery("feed_id", subscription.GetFeedID()),
			search.BoolQuery(
				search.WithBoolMinimumShouldMatch(1),
				search.WithBoolShould(
					search.BeforeQuery("published", before),
					search.BeforeQuery("updated", before),
				),
			),
		),
		search.WithBoolMustNot(
			search.TermsQuery("item_id", subscription.GetUnreadItems()),
		),
		search.WithBoolShould(
			search.TermsQuery("item_id", subscription.GetReadItems()),
		),
		ArticleFiltersQueryClause(subscription.GetArticleFilters()),
	)
}

// allSubscriptionFilters generates a set of query options to fetch all articles for the given subscriptions.
func allSubscriptionFilters(
	grouped map[time.Time][]models.FeedID,
	filtered models.Subscriptions,
	maxHistory time.Time,
) []search.QueryOption {
	filters := make([]search.QueryOption, 0)
	// Add filters for grouped subscriptions.
	filteredIDs := make([]models.FeedID, 0)
	for _, feedIDs := range grouped {
		filteredIDs = append(filteredIDs, feedIDs...)
	}
	filters = append(filters,
		search.BoolQuery(
			search.WithBoolFilter(
				search.TermsQuery("feed_id", filteredIDs),
			),
		),
	)
	// Add filters for subscriptions with article filters.
	for subscription := range slices.Values(filtered) {
		filters = append(filters, AllItemsForSubscriptionClause(subscription, maxHistory))
	}
	return filters
}

// AllItemsForSubscriptionClause generates a bool query that can be used to get all items for a subscription.
func AllItemsForSubscriptionClause(subscription *models.Subscription, maxHistory time.Time) search.QueryOption {
	return search.BoolQuery(
		search.WithBoolFilter(
			search.TermQuery("feed_id", subscription.GetFeedID()),
		),
		ArticleFiltersQueryClause(subscription.GetArticleFilters()),
	)
}

func ItemSortCombinations(sort *models.Sort) *search.Sorting {
	if sort == nil {
		return search.NewSortOptions(search.WithSortField("_doc"))
	}
	switch *sort {
	case models.SortNewestFirst:
		return search.NewSortOptions(
			search.WithSortField("published", search.WithFieldSortOrder(search.SortOrderDesc)),
			search.WithSortField("updated", search.WithFieldSortOrder(search.SortOrderDesc)),
			search.WithSortField("item_id", search.WithFieldSortOrder(search.SortOrderDesc)),
		)
	case models.SortOldestFirst:
		return search.NewSortOptions(
			search.WithSortField("published", search.WithFieldSortOrder(search.SortOrderAsc)),
			search.WithSortField("updated", search.WithFieldSortOrder(search.SortOrderAsc)),
			search.WithSortField("item_id", search.WithFieldSortOrder(search.SortOrderAsc)),
		)
	case models.SortMostRelevant:
		return search.NewSortOptions(
			search.WithSortField("_score", search.WithFieldSortOrder(search.SortOrderDesc)),
			search.WithSortField("published", search.WithFieldSortOrder(search.SortOrderDesc)),
			search.WithSortField("updated", search.WithFieldSortOrder(search.SortOrderDesc)),
			search.WithSortField("item_id", search.WithFieldSortOrder(search.SortOrderDesc)),
		)
	default:
		return search.NewSortOptions(
			search.WithSortField("_doc"),
		)
	}
}

func (s *ItemService) getItemContent(
	ctx context.Context,
	id models.ItemID,
	itemURL *url.URL,
) (*bytes.Buffer, error) {
	// Create a buffer for the feed data.
	itemContentBuf, ok := bufPool.Get().(*bytes.Buffer)
	if !ok {
		return nil, errors.New("get item content buffer failed")
	}
	itemContentBuf.Reset()
	defer bufPool.Put(itemContentBuf)

	// Try to load content from the article cache.
	if err := s.dataCache.Copy(ctx, id, itemContentBuf); err != nil {
		if apiErr, isAPIErr := errors.AsType[*models.APIError](err); isAPIErr {
			if apiErr.StatusCode != http.StatusNotFound {
				slogctx.FromCtx(ctx).Warn("Unable to copy article data from cache.",
					slog.Any("error", err),
				)
			}
		}
	}
	// If no item content cached, fetch from remote.
	if itemContentBuf.Len() == 0 {
		// Fetch the item's HTML source, used for enrichment.
		source, err := fetchItemContentDirect(ctx, s.httpClient, itemURL)
		if err != nil {
			return nil, fmt.Errorf("fetch item: %w", err)
		}
		if _, err := itemContentBuf.Write(source); err != nil {
			return nil, fmt.Errorf("write fetched item content to buffer: %w", err)
		}
	}
	return itemContentBuf, nil
}

func fetchItemContentDirect(ctx context.Context, httpClient *resty.Client, sourceURL *url.URL) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		slogctx.Warn(ctx, "Cannot fetch directly, context done.",
			slog.Any("cause", context.Cause(ctx)),
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("fetch: %w", err)
	}

	rawHTML, err := htmlx.GetHTML(ctx, httpClient, sourceURL.String())
	if err != nil {
		if respErr, isHtmlxErr := errors.AsType[*htmlx.Response](err); isHtmlxErr {
			// Check if response status is forbidden. If so, try through Zyte.
			if respErr.Status == http.StatusForbidden {
				if source, err := fetchItemContentThroughZyte(ctx, sourceURL); err != nil {
					return nil, fmt.Errorf("zyte fetch: %w", err)
				} else {
					return source, nil
				}
			}
			return nil, models.NewAPIError(respErr.Status, fmt.Errorf("direct fetch: %w", respErr))
		}
		return nil, models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("direct fetch: %w", err))
	}
	return rawHTML.Bytes(), nil
}

func fetchItemContentThroughZyte(ctx context.Context, sourceURL *url.URL) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		slogctx.Warn(ctx, "Cannot fetch item with Zyte, context done.",
			slog.Any("cause", context.Cause(ctx)),
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("cannot fetch item: %w", err)
	}

	switch extracted, err := zyte.Proxy(ctx,
		sourceURL.String(),
		zyte.WithResponseBody(true),
		zyte.WithFollowRedirects(true),
		zyte.WithTag("action", "enrich_item"),
	); {
	case err != nil:
		if zyteErr, isZyteErr := errors.AsType[*zyte.ResponseError](err); isZyteErr {
			return nil, models.NewAPIError(zyteErr.HTTPStatus(), zyteErr)
		}
		return nil, models.NewAPIError(http.StatusInternalServerError, err)
	case extracted == nil:
		return nil, models.NewAPIError(http.StatusInternalServerError, errors.New("no content extracted"))
	default:
		source, err := extracted.GetHTMLResponse()
		if err != nil {
			return nil, models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("get source: %w", err))
		}
		return source, nil
	}
}

func NewItemsFromZyteArticles(
	ctx context.Context,
	feed *models.Feed,
	articles *zyte.ArticleList,
) (models.Items, error) {
	items := make(models.Items, 0, len(articles.Articles))
	for article := range slices.Values(articles.Articles) {
		sourceURL, err := url.Parse(article.URL)
		if err != nil || !sourceURL.IsAbs() {
			slogctx.Warn(ctx, "Unable to parse article URL, skipping.",
				slog.String("url", article.URL),
			)
			continue
		}
		item := &models.Item{
			ItemID:      "item_" + strconv.FormatUint(xxh3.Hash([]byte(feed.GetID()+sourceURL.String())), 10),
			FeedID:      feed.GetID(),
			Timestamp:   time.Now().UTC(),
			Description: article.Description,
			SourceType:  feed.SourceType,
			URL:         sourceURL.String(),
			Language:    article.InLanguage,
			FeedTitle:   feed.GetTitle(),
		}
		if article.Headline != nil {
			item.Title = *article.Headline
		}
		item.Authors = make([]string, 0, len(article.Authors))
		for author := range slices.Values(article.Authors) {
			item.Authors = append(item.Authors, author.Name)
		}

		if article.GetContent() != "" {
			item.Content = new(validation.SanitizeHTML(article.GetContent()))
		}

		if pubDate, err := article.GetPublishedDate(); err != nil {
			item.Published = item.Timestamp
		} else {
			item.Published = pubDate.UTC()
		}
		if valid, _ := models.ValidateDatetime(item.Published); !valid {
			item.Published = feed.GetTimestamp()
		}
		if updDate, _ := article.GetUpdatedDate(); !updDate.IsZero() {
			updDateUTC := updDate.UTC()
			item.Updated = &updDateUTC
		}

		if article.MainImage != nil {
			item.Image = models.NewRemoteImage(article.MainImage.URL, item.Title)
		}

		if err := validation.Validate.Struct(item); err != nil {
			slogctx.Warn(ctx, "Invalid item. Ignoring",
				slog.String("feed_id", feed.GetID()),
				slog.String("item_id", item.GetID()),
				slog.Any("error", err),
			)
			continue
		}

		items = append(items, item)
	}

	return items, nil
}

// roundDownTo5Min truncates t to the most recent 5-minute boundary (floor), not the nearest one. For a "since" cutoff
// you almost always want floor, not round-to-nearest — rounding up risks skipping records that fall between the true
// cutoff and the rounded-up boundary.
func roundDownTo5Min(t time.Time) time.Time {
	return t.UTC().Truncate(5 * time.Minute)
}

// mostRecent returns the most recent of the two timestamps.
func mostRecent(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
