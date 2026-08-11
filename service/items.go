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
	"slices"
	"strconv"

	"codeberg.org/readeck/go-readability/v2"
	estypes "github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/sortorder"
	"github.com/maypok86/otter/v2"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/pkg/htmlx"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/retriever"
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
	newPagination, err := elastic.EncodePagination[models.Pagination](resp.Pagination)
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
	if pagination == nil || *pagination == "" {
		from = 0
	} else {
		var err error
		from, err = strconv.Atoi(*pagination)
		// searchAfter, err := elastic.DecodePagination(pagination)
		if err != nil {
			return nil, "", models.ErrInvalidParams
		}
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
		return nil, "", fmt.Errorf("search items: %w", err)
	}
	// Parse last search after value into pagination.
	newPagination := strconv.Itoa(from + count)
	return resp.Results, newPagination, nil
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
	return query.Bool(
		query.WithBoolQueryName(source.GetFeedID()+"_read_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", source.GetFeedID()),
			// And should be between the user max history and last read time.
			query.Bool(
				query.Should(
					query.Between("published", source.GetMarkedReadAt(), user.GetMaxHistory()),
					query.Between("updated", source.GetMarkedReadAt(), user.GetMaxHistory()),
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
		ArticleFiltersQueryClause(source.GetArticleFilters()),
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
		ArticleFiltersQueryClause(source.GetArticleFilters()),
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
		ArticleFiltersQueryClause(source.GetArticleFilters()),
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

	// Flag if the item needs enrichment.
	var needsEnriching bool
	if feed.Quirks != nil && feed.Quirks.FetchItemSummaries {
		needsEnriching = true
	}
	if item.GetImage() == nil {
		needsEnriching = true
	}
	// Bail if no enrichment needs to be done.
	if !needsEnriching {
		return nil
	}

	// Fetch the item's HTML source, used for enrichment.
	source, err := fetchItemDirect(ctx, item.GetLink())
	if err != nil {
		return err
	}

	// Extract any Opengraph data.
	opengraphData, err := htmlx.DecodeOpengraph(bytes.NewReader(source))
	if err != nil {
		slogctx.FromCtx(ctx).Debug("Unable to extract opengraph data for item.",
			slog.Any("error", err),
		)
	}
	// Extract readability data.
	readabilityData, err := readability.FromReader(bytes.NewReader(source), itemURL)
	if err != nil {
		slogctx.FromCtx(ctx).Debug("Unable to extract readability data for item.",
			slog.Any("error", err),
		)
	}

	// Add an image if needed.
	if item.GetImage() == nil {
		switch {
		case opengraphData != nil && opengraphData.Image != "":
			item.Image = models.NewRemoteImage(opengraphData.Image, item.GetTitle())
		case readabilityData.ImageURL() != "":
			item.Image = models.NewRemoteImage(readabilityData.ImageURL(), item.GetTitle())
		default:
			if imgURL, imgAlt, err := htmlx.ExtractImage(string(source), item.GetLink()); err != nil {
				slogctx.FromCtx(ctx).Debug("Unable to find suitable image for item.",
					slog.Any("error", err),
				)
			} else if imgURL != "" {
				item.Image = models.NewRemoteImage(imgURL, imgAlt)
			}
		}
	}

	// Add a description if needed.
	if feed.Quirks != nil && feed.Quirks.FetchItemSummaries {
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
