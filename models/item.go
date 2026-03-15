// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	estypes "github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/zeebo/xxh3"

	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/sortorder"
	"github.com/go-chi/chi/v5/middleware"
	feeds "github.com/immanent-tech/go-syndication"
	"github.com/immanent-tech/go-syndication/atom"
	"github.com/immanent-tech/go-syndication/opengraph"

	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/pkg/formats/html"
	"github.com/immanent-tech/foragd/pkg/formats/markdown"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/aggregations"
	"github.com/immanent-tech/foragd/providers/elastic/query"
)

func GetTopCategoriesForItems(ctx context.Context, itemsQueries ...query.Option) (CategoryCounts, error) {
	// Build aggregations.
	termsField := "categories.raw"
	termsCount := 200
	aggs := aggregations.Aggs{
		"CategoryCounts": estypes.Aggregations{
			Terms: &estypes.TermsAggregation{
				Field: &termsField,
				Size:  &termsCount,
			},
		},
	}

	resp, err := elastic.NewSearchRequest(
		elastic.WithRequestID[*search.Search, elastic.SearchRequest](middleware.GetReqID(ctx)),
		elastic.WithIndex[*search.Search, elastic.SearchRequest](schema.ItemsIndexRO),
		elastic.WithQueryOptions[*search.Search, elastic.SearchRequest](itemsQueries...),
		elastic.WithSize[*search.Search, elastic.SearchRequest](0),
		elastic.WithSortOptions[*search.Search, elastic.SearchRequest](
			&estypes.SortOptions{Doc_: estypes.NewScoreSort()},
		),
		elastic.WithAggregations[*search.Search, elastic.SearchRequest](aggs),
	).Do(ctx)
	if err != nil {
		return nil, ElasticsearchToAPIError(err)
	}

	categoryCounts, ok := resp.Aggregations["CategoryCounts"].(*estypes.StringTermsAggregate)
	if !ok {
		return nil, fmt.Errorf(
			"category counts aggregation invalid: %w",
			ErrInvalidAPIResult,
		)
	}
	categoryCountsBuckets, ok := categoryCounts.Buckets.([]estypes.StringTermsBucket)
	if !ok {
		return nil, fmt.Errorf(
			"unable to get feed stats: UnreadCounts aggregations invalid: %w",
			ErrInvalidAPIResult,
		)
	}

	counts := make(CategoryCounts, 0, len(categoryCountsBuckets))

	// Loop through the aggregation results and extract the unread count for each feed.
	for bucket := range slices.Values(categoryCountsBuckets) {
		var category Category
		if category, ok = bucket.Key.(string); ok {
			counts = append(counts, CategoryCount{Category: category, Count: int(bucket.DocCount)})
		}
	}
	return counts, nil
}

// CountItems returns a count of items that match the given query.
func CountItems(ctx context.Context, query query.Option) (int64, error) {
	count, err := elastic.Count(ctx, schema.ItemsIndexRO, query)
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
	sort *Sort,
	pagination *Pagination,
) (Items, Pagination, error) {
	searchAfter, err := elastic.DecodePagination(pagination)
	if err != nil {
		return nil, "", ErrInvalidParams
	}
	// Perform search.
	items, newSearchAfter, err := elastic.Search[Item](ctx, schema.ItemsIndexRO, query, count,
		elastic.WithSortOptions[*search.Search, elastic.SearchRequest](newItemSortOptions(sort)...),
		elastic.WithSearchAfter[*search.Search, elastic.SearchRequest](searchAfter...),
	)
	if err != nil {
		return nil, "", fmt.Errorf("search items: %w", err)
	}
	// Parse last search after value into pagination.
	newPagination, err := elastic.EncodePagination[Pagination](newSearchAfter)
	if err != nil {
		return nil, "", ErrInvalidParams
	}
	return items, newPagination, nil
}

// AddItems wraps an elastic bulk update to index items.
func AddItems(ctx context.Context, items ...Item) error {
	itemPtrs := make([]*Item, 0, len(items))
	for i := range slices.Values(items) {
		itemPtrs = append(itemPtrs, &i)
	}
	if _, err := elastic.BulkUpdate(ctx, schema.ItemsIndexRW, itemPtrs...); err != nil {
		return fmt.Errorf("add email item: %w", err)
	}
	return nil
}

// BuildItemsQuery generates a query to fetch the Items that match the given Filters from the given Subscriptions.
func BuildItemsQuery(
	ctx context.Context,
	filters Filters,
	subscriptionIDs ...SubscriptionID,
) (query.Option, error) {
	user := UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user data: %w", ErrCtxValueNotFound)
	}

	subscriptions, err := GetSubscriptions(ctx,
		GetSubscriptionsByIDs(subscriptionIDs...),
	)
	switch {
	case err != nil:
		return nil, fmt.Errorf("get suggestions: get subscriptions: %w", err)
	case len(subscriptions) == 0:
		return nil, fmt.Errorf("get suggestions: get subscriptions: %w", ErrNotFound)
	}

	// Search through items matching any given feeds filters, excluding any read
	// items.
	return query.Bool(
		query.WithBoolQueryName("get_items"),
		query.Filter(
			// Must match any of the given feed IDs.
			query.Terms("feed_id", subscriptions.GetFeedIDs()...),
			// Must match any of the given categories.
			query.Terms("categories.raw", filters.GetCategories()...),
			// And should match one feed clause.
			query.Bool(
				query.Should(BuildItemQueries(user, filters.GetView(), subscriptions)...),
			),
		),
	), nil
}

// ItemsAggregation performs an aggregation-only (i.e., search request with no hits returned) using the given query as
// the set of documents and performing the given aggregations across the documents.
func ItemsAggregation(
	ctx context.Context,
	query query.Option,
	size int,
	aggregations aggregations.Aggs,
) (*search.Response, error) {
	req := elastic.NewSearchRequest(
		elastic.WithRequestID[*search.Search, elastic.SearchRequest](middleware.GetReqID(ctx)),
		elastic.WithIndex[*search.Search, elastic.SearchRequest](schema.ItemsIndexRO),
		elastic.WithQueryOptions[*search.Search, elastic.SearchRequest](query),
		elastic.WithSize[*search.Search, elastic.SearchRequest](size),
		elastic.WithSortOptions[*search.Search, elastic.SearchRequest](
			&estypes.SortOptions{Doc_: estypes.NewScoreSort()},
		),
		elastic.WithAggregations[*search.Search, elastic.SearchRequest](aggregations),
	)
	resp, err := req.Do(ctx)
	if err != nil {
		return nil, ElasticsearchToAPIError(err)
	}

	return resp, nil
}

// Items is a slice of items.
type Items []Item

// FilterSince filters items to ones which are newer than the given timestamp.
func (i Items) FilterSince(since time.Time) Items {
	return slices.Collect(FilterSlice(i, func(item Item) bool {
		return item.IsNewer(since)
	}))
}

// FilterByFeed filters items to ones which match the given feed ID.
func (i Items) FilterByFeed(feedID FeedID) Items {
	return slices.Collect(FilterSlice(i, func(v Item) bool {
		return v.GetFeedID() == feedID
	}))
}

// GetFeedIDs retrieves a list of all FeedIDs from all items.
func (i Items) GetFeedIDs() []FeedID {
	feedIDs := make([]FeedID, 0, len(i))
	for item := range slices.Values(i) {
		feedIDs = append(feedIDs, item.GetFeedID())
	}
	return slices.Compact(feedIDs)
}

// GetIDs retrieves a list of all ItemIDs from all items.
func (i Items) GetIDs() []ItemID {
	itemIDs := make([]ItemID, 0, len(i))
	for item := range slices.Values(i) {
		itemIDs = append(itemIDs, item.GetID())
	}
	return slices.Compact(itemIDs)
}

// GetCategoryCounts returns a count of the occurrence of a Category across all
// the Items.
func (i Items) GetCategoryCounts() CategoryCounts {
	countsMap := make(map[Category]int)
	for item := range slices.Values(i) {
		for category := range slices.Values(item.GetCategories()) {
			countsMap[category]++
		}
	}
	var counts CategoryCounts
	for category, count := range maps.All(countsMap) {
		counts = append(counts, CategoryCount{Category: category, Count: count})
	}

	return counts
}

// SortByTimestamp sorts the items by their timestamps, in descending order.
func (i Items) SortByTimestamp() Items {
	slices.SortFunc(i, func(itemA, itemB Item) int {
		return itemA.GetTimestamp().Compare(itemB.GetTimestamp())
	})
	slices.Reverse(i)
	return i
}

// GetID returns the item ID.
func (i *Item) GetID() ItemID {
	return i.ItemID
}

// GetFeedID returns the ID of the feed the item belongs to.
func (i *Item) GetFeedID() FeedID {
	return i.FeedID
}

// GetLink returns the URL that should point to a page containing the full item content.
func (i *Item) GetLink() URL {
	return i.URL
}

// GetTitle returns the item's title.
func (i *Item) GetTitle() string {
	return i.Title
}

// GetDescription returns the summary of the item content, if any.
func (i *Item) GetDescription() string {
	if i.Description != nil {
		switch {
		case html.IsHTML(*i.Description):
			return *i.Description
		default:
			if formatted, err := markdown.ToHTML([]byte(*i.Description)); err != nil {
				return *i.Description
			} else {
				return string(formatted)
			}
		}
	}
	return ""
}

// GetAuthors returns a slice of the item's authors, if any.
func (i *Item) GetAuthors() []string {
	return i.Authors
}

// GetContributors returns a slice of the item's contributors, if any.
func (i *Item) GetContributors() []string {
	return i.Contributors
}

// GetCategories returns a slice of the item's categories, if any.
func (i *Item) GetCategories() []string {
	// Just in case there are duplicate categories, avoids a validation error.
	return slices.Compact(i.Categories)
}

// GetImage returns an image that can represent the item, if any.
func (i *Item) GetImage() *RemoteImage {
	return i.Image
}

// GetLanguage returns the language of the item, if set.
func (i *Item) GetLanguage() string {
	return i.Language
}

// GetRights returns the copyright associated with the item, if any.
func (i *Item) GetRights() string {
	return i.Copyright
}

// GetContent returns the full item content, if set.
func (i *Item) GetContent() string {
	return i.Content
}

// GetTimestamp returns a timestamp indicating when the item was last updated. This will be either, the updated
// timestamp, or, the published timestamp, or the indexing timestamp, whichever is found and
// is a valid value, in that order.
func (i *Item) GetTimestamp() time.Time {
	if valid, _ := validateDatetime(i.Updated); valid {
		return i.Updated.UTC()
	} else if valid, _ = validateDatetime(i.Published); valid {
		return i.Published.UTC()
	}
	return i.Timestamp.UTC()
}

// IsNewer returns a boolean indicating whether this item has been updated or
// published after the given time and before now (to ignore potentially incorrect dates in the future).
func (i *Item) IsNewer(since time.Time) bool {
	return i.GetTimestamp().After(since) && i.GetTimestamp().Before(time.Now().UTC())
}

// NewFeedItem generates an Item from the underlying feed data.
func NewFeedItem(ctx context.Context, source *feeds.Item, feed *Feed) *Item {
	// Generate a consistent document ID from either the item ID (if it has one) or the item URL.
	var itemID ItemID
	if sourceID := source.GetID(); sourceID != "" {
		itemID = "item_" + strconv.FormatUint(xxh3.Hash([]byte(feed.GetID()+sourceID)), 10)
	} else {
		itemID = "item_" + strconv.FormatUint(xxh3.Hash([]byte(feed.GetID()+source.GetLink())), 10)
	}
	item := &Item{
		ItemID:       itemID,
		FeedID:       feed.GetID(),
		Timestamp:    time.Now().UTC(),
		Published:    source.GetPublishedDate().UTC(),
		Updated:      source.GetUpdatedDate().UTC(),
		Title:        source.GetTitle(),
		Description:  new(source.GetDescription()),
		SourceType:   feed.SourceType,
		URL:          source.GetLink(),
		Authors:      source.GetAuthors(),
		Contributors: source.GetContributors(),
		Copyright:    source.GetRights(),
		Language:     source.GetLanguage(),
		Categories:   source.GetCategories(),
		Content:      source.GetContent(),
		FeedTitle:    feed.GetTitle(),
	}

	// Add youtube extension data if found.
	addYoutubeExtension(source, item)

	var wg sync.WaitGroup

	// Set the image.
	if sourceImg := source.GetImage(); sourceImg != nil {
		// Source has an image, use that.
		item.Image = &RemoteImage{
			URL:   new(sourceImg.GetURL()),
			Title: new(sourceImg.GetTitle()),
		}
	} else {
		wg.Go(func() {
			og, err := opengraph.ParseURL(ctx, source.GetLink())
			if err != nil {
				return
			}
			item.Image = &RemoteImage{
				URL: new(og.Image),
			}
		})
	}

	// Check for a valid published timestamp. If not valid, set the published timestamp to the feed's updated timestamp.
	if valid, _ := validateDatetime(item.Published); !valid {
		item.Published = feed.GetTimestamp()
	}

	return item
}

// NewEmailItem generates a new Item from an email.
func NewEmailItem(email Email, subscription *Subscription) *Item {
	// Generate a consistent document ID from either the item ID (if it has one) or the item URL.
	itemID := "item_" + strconv.FormatUint(xxh3.Hash([]byte(email.GetID())), 10)
	item := &Item{
		ItemID:     itemID,
		FeedID:     subscription.GetFeedID(),
		Timestamp:  email.Timestamp(),
		Published:  email.Timestamp(),
		Updated:    email.Timestamp(),
		Title:      email.GetSubject(),
		SourceType: SourceTypeEmail,
		Authors:    []string{email.GetFrom().String()},
		Content:    email.GetBody(),
		FeedTitle:  subscription.GetTitle(),
	}

	return item
}

// ItemSorting contains the sort options for sorting item search results.
type ItemSorting struct {
	Updated   string `json:"updated"`
	Published string `json:"published"`
	ItemID    string `json:"item_id"`
}

// SortCombinationsCaster is required to allow ItemSorting to be used as Elasticsearch sort values.
func (s *ItemSorting) SortCombinationsCaster() *estypes.SortCombinations {
	c := estypes.SortCombinations(s)
	return &c
}

func newItemSortOptions(sort *Sort) []estypes.SortCombinationsVariant {
	if sort == nil {
		return []estypes.SortCombinationsVariant{&estypes.SortOptions{Doc_: estypes.NewScoreSort()}}
	}
	var opts []estypes.SortCombinationsVariant
	switch *sort {
	case SortNewestFirst:
		opts = append(opts, &ItemSorting{
			Updated:   "desc",
			Published: "desc",
			ItemID:    "desc",
		})
	case SortOldestFirst:
		opts = append(opts, &ItemSorting{
			Updated:   "asc",
			Published: "asc",
			ItemID:    "asc",
		})
	case SortMostRelevant:
		opts = append(opts, &estypes.SortOptions{
			Score_: &estypes.ScoreSort{
				Order: &sortorder.Desc,
			},
		})
		opts = append(opts,
			&ItemSorting{
				Updated:   "asc",
				Published: "asc",
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

func newItemSortCombinations(sort *Sort) []estypes.SortCombinations {
	var opts []estypes.SortCombinations
	switch *sort {
	case SortNewestFirst:
		opts = append(opts, &ItemSorting{
			Updated:   "desc",
			Published: "desc",
			ItemID:    "desc",
		})
	case SortOldestFirst:
		opts = append(opts, &ItemSorting{
			Updated:   "asc",
			Published: "asc",
			ItemID:    "asc",
		})
	case SortMostRelevant:
		opts = append(opts, &estypes.SortOptions{
			Score_: &estypes.ScoreSort{
				Order: &sortorder.Desc,
			},
		})
		opts = append(opts,
			&ItemSorting{
				Updated:   "asc",
				Published: "asc",
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

func addYoutubeExtension(source *feeds.Item, item *Item) {
	// Extract and add additional information for youtube feeds.
	if strings.Contains(item.GetLink(), "youtube.com") && strings.HasPrefix(source.GetID(), "yt:video:") {
		if entry, ok := source.ItemSource.(*atom.Entry); ok {
			if len(entry.MediaGroup.Content) > 0 {
				width := entry.MediaGroup.Content[0].Width
				height := entry.MediaGroup.Content[0].Height
				videoID, ok := strings.CutPrefix(source.GetID(), "yt:video:")
				if ok {
					item.ExtensionType = new(ItemExtensionTypeYoutube)
					item.ExtensionData = &Item_ExtensionData{}
					item.ExtensionData.FromItemExtensionYoutube(ItemExtensionYoutube{
						VideoId: videoID,
						Width:   &width,
						Height:  &height,
					})
				}
			}

		}
	}
}
