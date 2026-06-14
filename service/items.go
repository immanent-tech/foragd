// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package service

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	estypes "github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/sortorder"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/maypok86/otter/v2"
	slogctx "github.com/veqryn/slog-context"
	"github.com/zeebo/xxh3"
	"golang.org/x/sync/errgroup"

	feeds "github.com/immanent-tech/go-syndication"
	"github.com/immanent-tech/go-syndication/atom"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/validation"
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

// SearchItems will search the items index for items matching the given query. Count, sort and pagination values are
// optional.
func SearchItems(
	ctx context.Context,
	query query.Option,
	count int,
	sort *models.Sort,
	pagination *models.Pagination,
) (models.Items, models.Pagination, error) {
	searchAfter, err := elastic.DecodePagination(pagination)
	if err != nil {
		return nil, "", models.ErrInvalidParams
	}
	// Perform search.
	resp, err := elastic.Search[*models.Item](ctx, schema.ItemsIndexRO(), query,
		elastic.WithSort(NewItemSortOptions(sort)...),
		elastic.WithSearchAfter(searchAfter...),
		elastic.WithSize(count),
	)
	if err != nil {
		return nil, "", fmt.Errorf("search items: %w", err)
	}
	// Parse last search after value into pagination.
	newPagination, err := elastic.EncodePagination[models.Pagination](resp.Pagination)
	if err != nil {
		return nil, "", models.ErrInvalidParams
	}
	return resp.Results, newPagination, nil
}

// AddItems wraps an elastic bulk update to index items.
func AddItems(ctx context.Context, items models.Items) (map[string]models.Items, error) {
	// Get any existing versions of the items.
	existingItems, err := GetItems(ctx, items.GetIDs()...)
	if err != nil {
		slogctx.FromCtx(ctx).
			Warn("Could not fetch existing items for comparing updates, falling back to bulk update of all items.",
				slog.Any("error", err),
			)
		if _, err := elastic.BulkUpdate(ctx, schema.ItemsIndexRW(), items...); err != nil {
			return nil, fmt.Errorf("bulk add items: %w", err)
		}
		return map[string]models.Items{"updated": items}, nil
	}

	// Collect updated items. Ignore noop updates like timestamp changes.
	// TODO: add a custom comparer when ExtensionData contains information worth updating.
	updatedItems := make(models.Items, 0, len(existingItems))
	for existingItem := range slices.Values(existingItems) {
		if newItem := items.FindByID(existingItem.GetID()); newItem != nil {
			if diff := cmp.Diff(*existingItem, *newItem,
				cmpopts.IgnoreFields(models.Item{}, "Updated", "Published", "Timestamp", "ExtensionData"),
				cmpopts.EquateEmpty(),
				cmpopts.IgnoreUnexported(),
			); diff != "" {
				updatedItems = append(updatedItems, newItem)
			}
		}
	}

	// Collect new items.
	newItems := items.ExcludeIDs(existingItems.GetIDs()...)

	wg, jobCtx := errgroup.WithContext(ctx)
	defer jobCtx.Done()
	results := make(map[string]models.Items)
	var mu sync.Mutex

	// Update updated items.
	if len(updatedItems) > 0 {
		wg.Go(func() error {
			if _, err := elastic.BulkUpdate(ctx, schema.ItemsIndexRW(), updatedItems...); err != nil {
				return fmt.Errorf("bulk update items: %w", err)
			}
			mu.Lock()
			defer mu.Unlock()
			results["updated"] = updatedItems
			return nil
		})
	}

	// Add new items.
	if len(newItems) > 0 {
		wg.Go(func() error {
			if _, err := elastic.BulkAdd(ctx, schema.ItemsIndexRW(), newItems...); err != nil {
				return fmt.Errorf("bulk add items: %w", err)
			}
			mu.Lock()
			defer mu.Unlock()
			results["new"] = newItems
			return nil
		})
	}

	if err := wg.Wait(); err != nil {
		return nil, fmt.Errorf("add/update items: %w", err)
	}

	return results, nil
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
		itemsQuery,
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
	// Work out what query to use based on the state filter.
	if len(subscriptions) == 0 {
		return nil
	}
	for subscription := range slices.Values(subscriptions) {
		// Ignore subscriptions that aren't based on a feed object.
		if subscription.GetFeedID() == "" {
			continue
		}

		switch view {
		case models.ViewRead:
			queries = append(queries, queryReadItems(user, subscription))
		case models.ViewAll:
			queries = append(queries, queryAllItems(user, subscription))
		case models.ViewUnread:
			fallthrough
		default:
			queries = append(queries, queryUnreadItems(user, subscription))
		}
	}
	return queries
}

// queryReadItems generates a query for finding read items for the given subscription.
func queryReadItems(user *models.User, source models.ItemSource) query.Option {
	// if subscription.GetSubscriptionType() != SubscriptionTypeFeed {
	// 	return nil
	// }
	return query.Bool(
		query.WithBoolQueryName(source.GetFeedID()+"_read_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", source.GetFeedID()),
			// And should be between the user max history and last read time.
			query.Bool(
				query.Should(
					query.Between("published", user.GetMaxHistory(), source.GetMarkedReadAt()),
					query.Between("updated", user.GetMaxHistory(), source.GetMarkedReadAt()),
					query.Terms("item_id", source.GetReadItems(), query.WithQueryName[*query.TermsQuery]("read-items")),
				),
				// Must not match any unread items for the feed
				query.MustNot(
					query.Terms(
						"item_id",
						source.GetUnreadItems(),
						query.WithQueryName[*query.TermsQuery]("unread-items"),
					),
				),
			),
		),
		// User-specified field-level filtering.
		models.ArticleFiltersQueryClause(source),
	)
}

// QueryUnreadItems generates a query for finding unread items for the given subscription.
func queryUnreadItems(_ *models.User, source models.ItemSource) query.Option {
	return query.Bool(
		query.WithBoolQueryName(source.GetFeedID()+"_unread_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", source.GetFeedID()),
			query.Bool(
				query.Should(
					query.Since("published", source.GetMarkedReadAt()),
					query.Since("updated", source.GetMarkedReadAt()),
					query.Terms(
						"item_id",
						source.GetUnreadItems(),
						query.WithQueryName[*query.TermsQuery]("unread-items"),
					),
				),
			),
		),
		// Must not match any read items for the feed
		query.MustNot(
			query.Terms("item_id", source.GetReadItems(), query.WithQueryName[*query.TermsQuery]("read-items")),
		),
		// User-specified field-level filtering.
		models.ArticleFiltersQueryClause(source),
	)
}

// subscriptionQueryReadItems generates a query for finding all items for the given subscription.
func queryAllItems(user *models.User, source models.ItemSource) query.Option {
	maxHistory := user.GetMaxHistory()
	return query.Bool(
		query.WithBoolQueryName(source.GetFeedID()+"_all_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", source.GetFeedID()),
			// And be published/updated since the user max history.
			query.Bool(
				query.Should(
					query.Since("published", maxHistory),
					query.Since("updated", maxHistory),
				),
			),
		),
		// User-specified field-level filtering.
		models.ArticleFiltersQueryClause(source),
	)
}

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

// NewFeedItem generates an Item from the underlying feed data.
func NewFeedItem(source *feeds.Item, feed *models.Feed) *models.Item {
	// Generate a consistent document ID from either the item ID (if it has one) or the item URL.
	var itemID models.ItemID
	if sourceID := source.GetID(); sourceID != "" {
		itemID = "item_" + strconv.FormatUint(xxh3.Hash([]byte(feed.GetID()+sourceID)), 10)
	} else {
		itemID = "item_" + strconv.FormatUint(xxh3.Hash([]byte(feed.GetID()+source.GetLink())), 10)
	}
	item := &models.Item{
		ItemID:       itemID,
		FeedID:       feed.GetID(),
		Timestamp:    time.Now().UTC(),
		Title:        source.GetTitle(),
		Description:  new(validation.SanitizeString(source.GetDescription())),
		SourceType:   feed.SourceType,
		URL:          source.GetLink(),
		Authors:      source.GetAuthors(),
		Contributors: source.GetContributors(),
		Copyright:    source.GetRights(),
		Language:     source.GetLanguage(),
		Categories:   source.GetCategories(),
		FeedTitle:    feed.GetTitle(),
	}
	if content := source.GetContent(); content != nil {
		item.Content = new(validation.SanitizeString(*content))
	}
	if pubDate := source.GetPublishedDate(); pubDate != nil {
		item.Published = pubDate.UTC()
	} else {
		item.Published = item.Timestamp
	}
	if updDate := source.GetUpdatedDate(); updDate != nil {
		item.Updated = new(updDate.UTC())
	}

	// Add youtube extension data if found.
	addYoutubeExtension(source, item)

	// Set the image.
	if sourceImg := source.GetImage(); sourceImg != nil {
		// Source has an image, use that.
		item.Image = models.NewRemoteImage(sourceImg.GetURL(), sourceImg.GetTitle())
	}

	// Check for a valid published timestamp. If not valid, set the published timestamp to the feed's updated timestamp.
	if valid, _ := models.ValidateDatetime(item.Published); !valid {
		item.Published = feed.GetTimestamp()
	}

	return item
}

// NewEmailItem generates a new Item from an email.
func NewEmailItem(email models.Email, subscription *models.Subscription) *models.Item {
	// Generate a consistent document ID from either the item ID (if it has one) or the item URL.
	itemID := "item_" + strconv.FormatUint(xxh3.Hash([]byte(email.GetID())), 10)
	item := &models.Item{
		ItemID:     itemID,
		FeedID:     subscription.GetFeedID(),
		Timestamp:  email.Timestamp(),
		Published:  email.Timestamp(),
		Updated:    new(email.Timestamp()),
		Title:      email.GetSubject(),
		SourceType: models.SourceTypeEmail,
		Authors:    []string{email.GetFrom().String()},
		Content:    new(email.GetBody()),
		FeedTitle:  subscription.GetTitle(),
	}

	return item
}

func addYoutubeExtension(source *feeds.Item, item *models.Item) {
	// Extract and add additional information for youtube feeds.
	if strings.Contains(item.GetLink(), "youtube.com") && strings.HasPrefix(source.GetID(), "yt:video:") {
		if entry, isValidEntry := source.ItemSource.(*atom.Entry); isValidEntry {
			if len(entry.MediaGroup.Content) > 0 {
				width := entry.MediaGroup.Content[0].Width
				height := entry.MediaGroup.Content[0].Height
				if videoID, isValidVideoID := strings.CutPrefix(source.GetID(), "yt:video:"); isValidVideoID {
					item.ExtensionType = new(models.ItemExtensionTypeYoutube)
					item.ExtensionData = &models.Item_ExtensionData{}
					item.ExtensionData.FromItemExtensionYoutube(models.ItemExtensionYoutube{
						VideoId: videoID,
						Width:   &width,
						Height:  &height,
					})
				}
			}

		}
	}
}
