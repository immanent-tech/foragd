// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"sync"
	"time"

	estypes "github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/sortorder"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/maypok86/otter/v2"
	slogctx "github.com/veqryn/slog-context"
	"github.com/zeebo/xxh3"

	"github.com/immanent-tech/go-base/config"
	"github.com/immanent-tech/go-base/pkg/htmlx"
	"github.com/immanent-tech/go-base/validation"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/retriever"
	"github.com/immanent-tech/foragd/providers/google/gcs"
	"github.com/immanent-tech/foragd/providers/zyte"
)

var itemsCache = otter.Must(&otter.Options[models.ItemID, *models.Item]{
	MaximumSize: 10_000,
})

// GetItems retrieves the Items matching the given ItemIDs.
func GetItems(ctx context.Context, ids ...models.ItemID) (models.Items, error) {
	var (
		items       models.Items
		unCachedIDs []models.ItemID
	)

	// Fetch items from cache.
	for id := range slices.Values(ids) {
		if item, found := itemsCache.GetIfPresent(id); found {
			items = append(items, item)
		} else {
			unCachedIDs = append(unCachedIDs, id)
		}
	}
	// If there are items missing from the cache, fetch and cache them.
	if len(unCachedIDs) > 0 {
		unCachedItems, err := elastic.GetDocs[models.ItemID, *models.Item](ctx, schema.ItemsIndexRO(), unCachedIDs...)
		if err != nil {
			return nil, fmt.Errorf("get items: %w", err)
		}
		for item := range slices.Values(unCachedItems) {
			items = append(items, item)
			itemsCache.Set(item.GetID(), item)
		}
	}
	return items, nil
}

// CountItems returns a count of items that match the given query.
func CountItems(ctx context.Context, query query.Option) (int64, error) {
	count, err := elastic.Count(ctx, schema.ItemsIndexRO(), query)
	if err != nil {
		return 0, fmt.Errorf("count items: %w", err)
	}

	return count, nil
}

// AddItems will add the given items to the database. It returns a map divided into "updated" and "new" items, to
// indicate items that existed and were updated vs. items that were added as new.
func AddItems(ctx context.Context, items models.Items) (map[string]models.Items, error) {
	// Get any existing versions of the items.
	existingItems, err := GetItems(ctx, items.GetIDs()...)
	if err != nil {
		slogctx.FromCtx(ctx).
			Warn("Could not fetch existing items for comparing updates, falling back to bulk update of all items.",
				slog.Any("error", err),
			)
		if err := bulk.IndexDocuments(ctx, schema.ItemsIndexRW(), items...); err != nil {
			return nil, models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("bulk add items: %w", err))
		}
		return map[string]models.Items{"updated": items}, nil
	}

	// Collect updated items. Ignore no-op updates like timestamp changes.
	// TODO: add a custom comparer when ExtensionData contains information worth updating.
	updatedItems := make(models.Items, 0, len(existingItems))
	for existingItem := range slices.Values(existingItems) {
		if updatedItem := items.FindByID(existingItem.GetID()); updatedItem != nil {
			if diff := cmp.Diff(
				*existingItem,
				*updatedItem,
				cmpopts.IgnoreFields(
					models.Item{},
					"Updated",
					"Published",
					"Timestamp",
					"ExtensionData",
					"ExtensionType",
				),
				cmpopts.EquateEmpty(),
				cmpopts.IgnoreUnexported(),
			); diff != "" {
				updatedItems = append(updatedItems, updatedItem)
			}
		}
	}

	// Collect new items.
	newItems := items.ExcludeIDs(existingItems.GetIDs()...)

	// Index all items.
	results := make(map[string]models.Items)
	results["updated"] = updatedItems
	results["new"] = newItems
	if err := bulk.IndexDocuments(ctx, schema.ItemsIndexRW(), slices.Concat(updatedItems, newItems)...); err != nil {
		return nil, models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("bulk add/update items: %w", err))
	}

	if err := bulk.Flush(ctx); err != nil {
		slogctx.Warn(ctx, "Flush bulk request failed.",
			slog.Any("error", err))
	}

	return results, nil
}

// SearchItems will search the items index for items matching the given query. Count, sort and pagination values are
// optional.
func SearchItems(
	ctx context.Context,
	query query.Option,
	count int,
	sort *models.Sort,
	pagination *string,
) (models.Items, string, error) {
	searchAfter, err := elastic.DecodePagination(pagination)
	if err != nil {
		return nil, "", models.ErrInvalidParams
	}
	// Perform search.
	resp, err := elastic.Search[*models.Item](ctx,
		schema.ItemsIndexRO(),
		elastic.WithQueryOptions[*elastic.SearchRequest](query),
		elastic.WithSort(NewItemSortOptions(sort)...),
		elastic.WithSearchAfter(searchAfter...),
		elastic.WithSize(count),
	)
	if err != nil {
		return nil, "", fmt.Errorf("search items: %w", err)
	}
	// Parse last search after value into pagination.
	newPagination, err := elastic.EncodePagination[string](resp.Pagination)
	if err != nil {
		return nil, "", models.ErrInvalidParams
	}
	return resp.Results, newPagination, nil
}

func RetrieveItems(
	ctx context.Context,
	retriever retriever.Option,
	count int,
	pagination *models.Pagination,
) (models.Items, models.Pagination, error) {
	var from int
	if pagination.From == nil {
		from = 0
	} else {
		from = *pagination.From
	}

	// Perform search.
	resp, err := elastic.Search[*models.Item](ctx,
		schema.ItemsIndexRO(),
		elastic.WithRetriever(retriever),
		// elastic.WithSort(NewItemSortOptions(sort)...),
		elastic.WithFrom(from),
		elastic.WithSize(count),
	)
	if err != nil {
		return nil, models.Pagination{}, fmt.Errorf("search items: %w", err)
	}
	// Parse last search after value into pagination.
	return resp.Results, models.Pagination{From: new(from + count)}, nil
}

func GetTopCategoriesForItems(ctx context.Context, itemsQuery query.Option) (models.CategoryCounts, error) {
	// Build elastic.
	termsField := "categories.raw"
	termsCount := 200
	aggs := elastic.Aggs{
		"CategoryCounts": estypes.Aggregations{
			Terms: &estypes.TermsAggregation{
				Field: &termsField,
				Size:  &termsCount,
			},
		},
	}

	resp, err := elastic.Search[*models.Item](ctx,
		schema.ItemsIndexRO(),
		elastic.WithQueryOptions[*elastic.SearchRequest](itemsQuery),
		elastic.WithAggregations(aggs),
		elastic.WithSize(0),
		elastic.WithDocSorting(),
	)
	if err != nil {
		return nil, ElasticsearchToAPIError(err)
	}

	categoryCounts, ok := resp.Aggregations["CategoryCounts"].(*estypes.StringTermsAggregate)
	if !ok {
		return nil, fmt.Errorf(
			"category counts aggregation invalid: %w",
			models.ErrInvalidAPIResult,
		)
	}
	categoryCountsBuckets, ok := categoryCounts.Buckets.([]estypes.StringTermsBucket)
	if !ok {
		return nil, fmt.Errorf(
			"unable to get feed stats: UnreadCounts aggregations invalid: %w",
			models.ErrInvalidAPIResult,
		)
	}

	counts := make(models.CategoryCounts, 0, len(categoryCountsBuckets))

	// Loop through the aggregation results and extract the unread count for each feed.
	for bucket := range slices.Values(categoryCountsBuckets) {
		var category models.Category
		if category, ok = bucket.Key.(string); ok {
			counts = append(counts, models.CategoryCount{Category: category, Count: int(bucket.DocCount)})
		}
	}
	return counts, nil
}

// BuildItemQueries generates a slices of queries for the given subscriptions, based on the given filters.
func BuildItemQueries(
	user *models.User,
	view models.View,
	subscriptions models.Subscriptions,
) []query.Option {
	queries := make([]query.Option, 0, len(subscriptions))

	// Ignore if no subscriptions specified.
	if len(subscriptions) == 0 {
		return nil
	}

	// Filter to favorites.
	if view == models.ViewFavorites {
		return []query.Option{
			query.Terms(
				"item_id",
				user.ItemFavorites,
				query.WithQueryName[*query.TermsQuery]("favorite-items"),
			),
		}
	}

	// Filter based on read/unread/all.
	grouped := make(map[string][]models.FeedID)
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
			tsIdx := roundDownTo5Min(subscription.GetMarkedReadAt()).Format("2006-01-02T15:04:05Z")
			feedIDs := grouped[tsIdx]
			feedIDs = append(feedIDs, subscription.GetFeedID())
			grouped[tsIdx] = feedIDs
		}
	}

	switch view {
	case models.ViewAll:
		// queries = append(queries, queryAllItems(user, subscription))
		filters := allSubscriptionFilters(grouped, filtered, user.GetMaxHistory())
		query := query.Bool(
			query.WithBoolShouldMinimumMatch(1),
			query.Should(filters...),
		)
		queries = append(queries, query)
	case models.ViewRead:
		// queries = append(queries, queryReadItems(user, subscription))
		filters := readSubscriptionFilters(grouped, filtered, user.GetMaxHistory())
		query := query.Bool(
			query.WithBoolShouldMinimumMatch(1),
			query.Should(filters...),
			query.Should(query.Terms("item_id", readItems)),
			query.MustNot(query.Terms("item_id", unreadItems)),
		)
		queries = append(queries, query)
	case models.ViewUnread:
		fallthrough
	default:
		// queries = append(queries, queryUnreadItems(user, subscription))
		filters := unReadSubscriptionFilters(grouped, filtered)
		query := query.Bool(
			query.WithBoolShouldMinimumMatch(1),
			query.Should(filters...),
			query.Should(query.Terms("item_id", unreadItems)),
			query.MustNot(query.Terms("item_id", readItems)),
		)
		queries = append(queries, query)
	}

	return queries
}

// unReadSubscriptionFilters generates a set of query options to fetch unread articles for the given subscriptions.
func unReadSubscriptionFilters(grouped map[string][]models.FeedID, filtered models.Subscriptions) []query.Option {
	filters := make([]query.Option, 0)
	// Add filters for grouped subscriptions.
	for ts, feedIDs := range grouped {
		filters = append(filters,
			query.Bool(
				query.Filter(
					query.Terms("feed_id", feedIDs),
					query.Bool(
						query.WithBoolShouldMinimumMatch(1),
						query.Should(
							query.Since("published", ts),
							query.Since("updated", ts),
						),
					),
				),
			),
		)
	}
	// Add filters for subscriptions with article filters.
	for subscription := range slices.Values(filtered) {
		filters = append(filters, unreadItemsForSubscriptionClause(subscription))
	}
	return filters
}

// unreadItemsForSubscriptionClause generates a bool query that can be used to get unread items for a subscription.
func unreadItemsForSubscriptionClause(subscription *models.Subscription) query.Option {
	return query.Bool(
		query.WithBoolQueryName(subscription.GetID()+"_unread"),
		query.Filter(
			query.Term("feed_id", subscription.GetFeedID()),
			query.Bool(
				query.WithBoolShouldMinimumMatch(1),
				query.Should(
					query.Since("published", subscription.GetMarkedReadAt().Format("2006-01-02T15:04:05Z")),
					query.Since("updated", subscription.GetMarkedReadAt().Format("2006-01-02T15:04:05Z")),
				),
			),
		),
		ArticleFiltersQueryClause(subscription.GetArticleFilters()),
	)
}

// readSubscriptionFilters generates a set of query options to fetch read articles for the given subscriptions.
func readSubscriptionFilters(
	grouped map[string][]models.FeedID,
	filtered models.Subscriptions,
	maxHistory time.Time,
) []query.Option {
	filters := make([]query.Option, 0)
	// Add filters for grouped subscriptions.
	for ts, feedIDs := range grouped {
		filters = append(filters,
			query.Bool(
				query.Filter(
					query.Terms("feed_id", feedIDs),
					query.Bool(
						query.WithBoolShouldMinimumMatch(1),
						query.Should(
							query.Between("published", ts, maxHistory.Format("2006-01-02T15:04:05Z")),
							query.Between("updated", ts, maxHistory.Format("2006-01-02T15:04:05Z")),
						),
					),
				),
			),
		)
	}
	// Add filters for subscriptions with article filters.
	for subscription := range slices.Values(filtered) {
		filters = append(filters, readItemsForSubscriptionClause(subscription, maxHistory))
	}
	return filters
}

// readItemsForSubscriptionClause generates a bool query that can be used to get read items for a subscription.
func readItemsForSubscriptionClause(subscription *models.Subscription, maxHistory time.Time) query.Option {
	return query.Bool(
		query.Filter(
			query.Term("feed_id", subscription.GetFeedID()),
			query.Bool(
				query.WithBoolShouldMinimumMatch(1),
				query.Should(
					query.Between(
						"published",
						subscription.GetMarkedReadAt().Format("2006-01-02T15:04:05Z"),
						maxHistory.Format("2006-01-02T15:04:05Z"),
					),
					query.Between(
						"updated",
						subscription.GetMarkedReadAt().Format("2006-01-02T15:04:05Z"),
						maxHistory.Format("2006-01-02T15:04:05Z"),
					),
				),
			),
		),
		ArticleFiltersQueryClause(subscription.GetArticleFilters()),
	)
}

// allSubscriptionFilters generates a set of query options to fetch all articles for the given subscriptions.
func allSubscriptionFilters(
	grouped map[string][]models.FeedID,
	filtered models.Subscriptions,
	maxHistory time.Time,
) []query.Option {
	filters := make([]query.Option, 0)
	// Add filters for grouped subscriptions.
	filteredIDs := make([]models.FeedID, 0)
	for _, feedIDs := range grouped {
		filteredIDs = append(filteredIDs, feedIDs...)
	}
	filters = append(filters,
		query.Bool(
			query.Filter(
				query.Terms("feed_id", filteredIDs),
				query.Bool(
					query.WithBoolShouldMinimumMatch(1),
					query.Should(
						query.Since("published", maxHistory.Format("2006-01-02T15:04:05Z")),
						query.Since("updated", maxHistory.Format("2006-01-02T15:04:05Z")),
					),
				),
			),
		),
	)
	// Add filters for subscriptions with article filters.
	for subscription := range slices.Values(filtered) {
		filters = append(filters, allItemsForSubscriptionClause(subscription, maxHistory))
	}
	return filters
}

// allItemsForSubscriptionClause generates a bool query that can be used to get all items for a subscription.
func allItemsForSubscriptionClause(subscription *models.Subscription, maxHistory time.Time) query.Option {
	return query.Bool(
		query.Filter(
			query.Term("feed_id", subscription.GetFeedID()),
			query.Bool(
				query.WithBoolShouldMinimumMatch(1),
				query.Should(
					query.Since("published", maxHistory.Format("2006-01-02T15:04:05Z")),
					query.Since("updated", maxHistory.Format("2006-01-02T15:04:05Z")),
				),
			),
		),
		ArticleFiltersQueryClause(subscription.GetArticleFilters()),
	)
}

// // queryReadItems generates a query for finding read items for the given subscription.
// func queryReadItems(user *models.User, source models.ItemSource) query.Option {
// 	return query.Bool(
// 		query.WithBoolQueryName(source.GetFeedID()+"_read_items"),
// 		query.Filter(
// 			// Must match this feed.
// 			query.Term("feed_id", source.GetFeedID()),
// 			// And should be between the user max history and last read time.
// 			query.Bool(
// 				query.Should(
// 					query.Between("published", source.GetMarkedReadAt(), user.GetMaxHistory()),
// 					query.Between("updated", source.GetMarkedReadAt(), user.GetMaxHistory()),
// 					query.Terms("item_id", source.GetReadItems(), query.WithQueryName[*query.TermsQuery]("read-items")),
// 				),
// 				// Must not match any unread items for the feed
// 				query.MustNot(
// 					query.Terms(
// 						"item_id",
// 						source.GetUnreadItems(),
// 						query.WithQueryName[*query.TermsQuery]("unread-items"),
// 					),
// 				),
// 			),
// 		),
// 		// User-specified field-level filtering.
// 		ArticleFiltersQueryClause(source.GetArticleFilters()),
// 	)
// }

// // QueryUnreadItems generates a query for finding unread items for the given subscription.
// func queryUnreadItems(_ *models.User, source models.ItemSource) query.Option {

// 	return query.Bool(
// 		query.WithBoolQueryName(source.GetFeedID()+"_unread_items"),
// 		query.Filter(
// 			// Must match this feed.
// 			query.Term("feed_id", source.GetFeedID()),
// 			query.Bool(
// 				query.Should(
// 					query.Since("published", source.GetMarkedReadAt()),
// 					query.Since("updated", source.GetMarkedReadAt()),
// 					query.Terms(
// 						"item_id",
// 						source.GetUnreadItems(),
// 						query.WithQueryName[*query.TermsQuery]("unread-items"),
// 					),
// 				),
// 			),
// 		),
// 		// Must not match any read items for the feed
// 		query.MustNot(
// 			query.Terms("item_id", source.GetReadItems(), query.WithQueryName[*query.TermsQuery]("read-items")),
// 		),
// 		// User-specified field-level filtering.
// 		ArticleFiltersQueryClause(source.GetArticleFilters()),
// 	)
// }

// // subscriptionQueryReadItems generates a query for finding all items for the given subscription.
// func queryAllItems(user *models.User, source models.ItemSource) query.Option {
// 	maxHistory := user.GetMaxHistory()
// 	return query.Bool(
// 		query.WithBoolQueryName(source.GetFeedID()+"_all_items"),
// 		query.Filter(
// 			// Must match this feed.
// 			query.Term("feed_id", source.GetFeedID()),
// 			// And be published/updated since the user max history.
// 			query.Bool(
// 				query.Should(
// 					query.Since("published", maxHistory),
// 					query.Since("updated", maxHistory),
// 				),
// 			),
// 		),
// 		// User-specified field-level filtering.
// 		ArticleFiltersQueryClause(source.GetArticleFilters()),
// 	)
// }

// ItemSorting contains the sort options for sorting item search results.
type ItemSorting struct {
	Published string `json:"published"`
	Updated   string `json:"updated"`
	ItemID    string `json:"item_id"`
}

// SortCombinationsCaster is required to allow ItemSorting to be used as Elasticsearch sort values.
func (s *ItemSorting) SortCombinationsCaster() *estypes.SortCombinations {
	c := estypes.SortCombinations(s)
	return &c
}

func NewItemSortOptions(sort *models.Sort) []estypes.SortCombinationsVariant {
	if sort == nil {
		return []estypes.SortCombinationsVariant{&estypes.SortOptions{Doc_: estypes.NewScoreSort()}}
	}
	var opts []estypes.SortCombinationsVariant
	switch *sort {
	case models.SortNewestFirst:
		opts = append(opts, &ItemSorting{
			Published: "desc",
			Updated:   "desc",
			ItemID:    "desc",
		})
	case models.SortOldestFirst:
		opts = append(opts, &ItemSorting{
			Published: "asc",
			Updated:   "asc",
			ItemID:    "asc",
		})
	case models.SortMostRelevant:
		opts = append(opts, &estypes.SortOptions{
			Score_: &estypes.ScoreSort{
				Order: &sortorder.Desc,
			},
		})
		opts = append(opts,
			&ItemSorting{
				Published: "asc",
				Updated:   "asc",
				ItemID:    "asc",
			},
		)
	default:
		opts = append(opts, &estypes.SortOptions{
			Doc_: &estypes.ScoreSort{},
		})
	}
	return opts
}

func NewItemSortCombinations(sort *models.Sort) []estypes.SortCombinations {
	var opts []estypes.SortCombinations
	switch *sort {
	case models.SortNewestFirst:
		opts = append(opts, &ItemSorting{
			Published: "desc",
			Updated:   "desc",
			ItemID:    "desc",
		})
	case models.SortOldestFirst:
		opts = append(opts, &ItemSorting{
			Published: "asc",
			Updated:   "asc",
			ItemID:    "asc",
		})
	case models.SortMostRelevant:
		opts = append(opts, &estypes.SortOptions{
			Score_: &estypes.ScoreSort{
				Order: &sortorder.Desc,
			},
		})
		opts = append(opts,
			&ItemSorting{
				Published: "asc",
				Updated:   "asc",
				ItemID:    "asc",
			},
		)
	default:
		opts = append(opts, &estypes.SortOptions{
			Doc_: estypes.NewScoreSort(),
		})
	}
	return opts
}

// EnrichItem checks the item data if it is missing certain values, flags it, then tries to enrich the item to fill
// missing data from the item source.
func EnrichItem(ctx context.Context, feed *models.Feed, item *models.Item) error {
	ctx = slogctx.With(ctx,
		slog.String("item_id", item.GetID()),
		slog.String("feed_id", item.GetFeedID()),
	)

	itemURL, err := url.Parse(item.GetLink())
	if err != nil {
		return models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("parse item link: %w", err))
	}

	var fetchSummaries bool

	// Check if the feed indicates summaries should be fetched separately.
	if feed.FetchMethod == models.FeedFetchMethodDirect || feed.FetchMethod == models.FeedFetchMethodProxied {
		if feed.FetchOptions != nil {
			if fetchOptions, err := feed.FetchOptions.AsFetchDirectOptions(); err != nil {
				slogctx.Warn(ctx, "Unable to parse feed fetch options.",
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
	itemContentBuf, err := getItemContent(ctx, item)
	if err != nil {
		return models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("get item content: %w", err))
	}

	// Extract opengraph and readability data from item HTML source.
	opengraphData, readabilityData, err := extractMetadataFromHTML(itemURL, itemContentBuf.Bytes())
	if err != nil {
		logGeneralError(ctx, err, feed.GetSourceURLs()[0], feed.GetID())
	}

	// Add an image if needed.
	if item.GetImage() == nil {
		switch {
		case opengraphData != nil && opengraphData.Image != "":
			item.Image = models.NewRemoteImage(opengraphData.Image, item.GetTitle())
		case readabilityData.ImageURL() != "":
			item.Image = models.NewRemoteImage(readabilityData.ImageURL(), item.GetTitle())
		default:
			if imgURL, imgAlt, _ := htmlx.ExtractImage(itemContentBuf.String(), item.GetLink()); imgURL != "" {
				item.Image = models.NewRemoteImage(imgURL, imgAlt)
			}
		}
	}

	// When item summaries needed to be fetched, check and add an item description if missing.
	if fetchSummaries {
		switch {
		case opengraphData != nil && opengraphData.Description != "":
			item.Description = &opengraphData.Description
		case readabilityData.Excerpt() != "":
			desc := readabilityData.Excerpt()
			item.Description = &desc
		}
	}

	return nil
}

func getItemContent(ctx context.Context, item *models.Item) (*bytes.Buffer, error) {
	// Create a buffer for the feed data.
	itemContentBuf, ok := bufPool.Get().(*bytes.Buffer)
	if !ok {
		return nil, errors.New("get item content buffer failed")
	}
	itemContentBuf.Reset()
	defer bufPool.Put(itemContentBuf)

	// Try to load content from the article cache.
	if err := loaditemPageCache(); err != nil {
		slogctx.FromCtx(ctx).Debug("Unable to load item content cache.",
			slog.Any("error", err),
		)
	} else {
		if err := itemPageCache.Copy(ctx, item.GetID(), itemContentBuf); err != nil {
			if apiErr, isAPIErr := errors.AsType[*models.APIError](err); isAPIErr {
				if apiErr.StatusCode != http.StatusNotFound {
					slogctx.FromCtx(ctx).Warn("Unable to copy article data from cache.",
						slog.Any("error", err),
					)
				}
			}
		}
	}
	// If no item content cached, fetch from remote.
	if itemContentBuf.Len() == 0 {
		// Fetch the item's HTML source, used for enrichment.
		source, err := fetchItemDirect(ctx, item.GetLink())
		if err != nil {
			return nil, fmt.Errorf("fetch item: %w", err)
		}
		if _, err := itemContentBuf.Write(source); err != nil {
			return nil, fmt.Errorf("write fetched item content to buffer: %w", err)
		}
	}
	return itemContentBuf, nil
}

func fetchItemDirect(ctx context.Context, link string) ([]byte, error) {
	rawHTML, err := htmlx.GetHTML(ctx, link)
	if err != nil {
		if respErr, isHtmlxErr := errors.AsType[*htmlx.Response](err); isHtmlxErr {
			// Check if response status is forbidden. If so, try through Zyte.
			if respErr.Status == http.StatusForbidden {
				if source, err := fetchItemThroughZyte(ctx, link); err != nil {
					return nil, fmt.Errorf("fetch item: %w", err)
				} else {
					return source, nil
				}
			}
			return nil, models.NewAPIError(respErr.Status, fmt.Errorf("fetch item: %w", respErr))
		}
		return nil, models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("fetch item: %w", err))
	}
	return rawHTML.Bytes(), nil
}

func fetchItemThroughZyte(ctx context.Context, link string) ([]byte, error) {
	switch extracted, err := zyte.Proxy(ctx,
		link,
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
		item := &models.Item{
			ItemID:      "item_" + strconv.FormatUint(xxh3.Hash([]byte(feed.GetID()+article.URL)), 10),
			FeedID:      feed.GetID(),
			Timestamp:   time.Now().UTC(),
			Description: article.Description,
			SourceType:  feed.SourceType,
			URL:         article.URL,
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
			item.Content = new(validation.SanitizeString(article.GetContent()))
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

var itemPageCache objectCache

var loaditemPageCache = sync.OnceValue(func() error {
	switch config.GetEnvironment() {
	case config.EnvProduction:
		bucketName := os.Getenv("FORAGD_SERVER_BUCKET")
		var err error
		itemPageCache, err = gcs.Connect(context.Background(), bucketName, "articles")
		if err != nil {
			return fmt.Errorf("connect to gcs: %w", err)
		}
	default:
		var err error
		itemPageCache, err = newDirCache("articles")
		if err != nil {
			return fmt.Errorf("create dir cache: %w", err)
		}
	}

	return nil
})

// roundDownTo5Min truncates t to the most recent 5-minute boundary (floor), not the nearest one. For a "since" cutoff
// you almost always want floor, not round-to-nearest — rounding up risks skipping records that fall between the true
// cutoff and the rounded-up boundary.
func roundDownTo5Min(t time.Time) time.Time {
	return t.UTC().Truncate(5 * time.Minute)
}
