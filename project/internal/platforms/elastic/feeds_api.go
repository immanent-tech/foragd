// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"log/slog"

	"github.com/elastic/go-elasticsearch/v8/typedapi"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/count"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/refresh"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/sortorder"

	"github.com/joshuar/go-feed-me/internal/api"
	"github.com/joshuar/go-feed-me/internal/id"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/query"
)

var defaultFeedFields = []string{
	"publishedParsed",
	"updatedParsed",
	"feed_id",
	"title",
	"description",
	"feedLink",
	"image",
	"categories",
	"authors",
}

var defaultItemFields = []string{
	"publishedParsed",
	"updatedParsed",
	"title",
	"description",
	"item_id",
	"image",
}

var defaultDatetimeFormat = "strict_date_optional_time_nanos"

var (
	ErrNoFeedID  = errors.New("no feed ID provided")
	ErrAddFailed = errors.New("adding items failed")
)

func FeedExists(ctx context.Context, esapi *typedapi.API, value string) (bool, *api.Error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return false, ErrFetchCtx
	}

	var queryClause query.Option

	switch {
	case id.IdentifyID(value) == id.Feed:
		queryClause = query.FeedIDs(value)
	default:
		queryClause = query.Term("feedLink", value)
	}

	resp, err := NewSearchRequest(esapi,
		WithSearchIndex(index),
		WithSearchQueryOptions(queryClause),
		WithSortOptions(SortByDocID("feed_id")),
	).Do(ctx)
	if err != nil {
		return false, api.WrapError(ErrSearchFailed, "elastic", "backend error occurred")
	}

	if resp.Hits.Total.Value == 0 {
		return false, nil
	}

	return true, nil
}

func GetFeedByURL(ctx context.Context, api *typedapi.API, url string) (*models.APIFeed, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	resp, err := NewSearchRequest(api,
		WithSearchIndex(index),
		WithFields(defaultFeedFields...),
		WithSearchQueryOptions(query.Term("feedLink", url)),
		WithSortOptions(SortByDocID("feed_id")),
	).Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrSearchFailed, err)
	}

	// If there are no hits, just return an empty APIFeed object.
	if resp.Hits.Total.Value == 0 {
		return nil, models.ErrNoFeed
	}

	feed, err := ExtractSource[*models.APIFeed](resp.Hits.Hits[0].Source_)
	if err != nil {
		return nil, errors.Join(ErrSearchFailed, err)
	}

	return feed, nil
}

// GetFeedsByURL retrieves a list of APIFeeds based on the given URLs.
func GetFeedsByURL(ctx context.Context, esapi *typedapi.API, urls ...models.URL) ([]models.APIFeed, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, ErrFetchCtx
	}

	feeds := make([]models.APIFeed, 0, len(urls))

	resp, err := NewSearchRequest(esapi,
		WithSearchIndex(index),
		WithFields("feed_id", "feedLink"),
		WithSearchQueryOptions(query.URLs("feedLink", urls...)),
		WithSearchSize(len(urls)),
		WithSortOptions(SortByDocID("feed_id")),
	).Do(ctx)
	if err != nil {
		return nil, api.WrapError(ErrSearchFailed, "elastic", "backend request failed")
	}
	// Stop if there are no hits
	if len(resp.Hits.Hits) == 0 {
		return nil, nil
	}
	// Loop through this set of results.
	sources, _, warnings := ExtractSourceFromHits[models.APIFeed](resp.Hits.Hits)
	if warnings != nil {
		logging.FromContext(ctx).Warn("Problems occurred while extracting source from docs.",
			slog.Any("warnings", err))
	}

	feeds = append(feeds, sources...)

	return feeds, nil
}

// FeedsSearch searches the feeds index for feeds matching the relevant filters.
func FeedsSearch(ctx context.Context, api *typedapi.API, filters api.Filters) ([]*models.APIFeed, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	resp, err := NewSearchRequest(api,
		WithSearchIndex(index),
		WithSearchQueryOptions(
			query.Bool(
				// Match either the FeedID OR the Category.
				query.Should(
					query.FeedIDs(filters.GetFeeds()...),
					query.Categories(filters.GetCategories()...),
				),
			),
		),
		WithSearchSize(filters.GetCount()),
		WithSortOptions(setFeedSort(filters.GetSort())),
	).Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrSearchFailed, err)
	}
	// Stop if there are no hits
	if len(resp.Hits.Hits) == 0 {
		return nil, nil
	}
	// Loop through this set of results.
	sources, _, warnings := ExtractSourceFromHits[*models.APIFeed](resp.Hits.Hits)
	if warnings != nil {
		logging.FromContext(ctx).Warn("Problems occurred while extracting source from docs.",
			slog.Any("warnings", err))
	}

	return sources, nil
}

func GetFeedCategories(ctx context.Context, api *typedapi.API, feedIDs ...models.FeedID) (*TermsAggregationResults, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	// Find all feeds with the given IDs, build a terms aggregation on the
	// categories field.
	req := NewSearchRequest(api,
		WithSearchIndex(index),
		WithSearchQueryOptions(
			query.FeedIDs(feedIDs...),
		),
		WithSortOptions(defaultFeedSort()),
		WithSearchSize(0),
		WithAggregations(NewTermsAggregation("Categories", "categories.raw")),
	)

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrUserActionFailed, err)
	}

	var results TermsAggregationResults

	results.StringTermsAggregate, err = ExtractAggregation[*types.StringTermsAggregate](resp, "Categories")
	if err != nil {
		return nil, errors.Join(ErrUserActionFailed, err)
	}

	// categoryCounts := make(map[string]int64)
	// for _, feedID := range feedIDs {
	// 	categoryCounts[feedID] = results.GetCount(feedID)
	// }

	return &results, nil
}

// GetFeedsByID retrieves a list of feeds by their FeedID. If the list of feeds
// needs filtering, this should be done before calling this method as an mget
// request offers no filtering options.
func (c *Client) GetFeedsByID(ctx context.Context, feedIDs ...models.FeedID) ([]*models.APIFeed, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	// Get the feed details.
	req := NewMGetRequest(c.GetAPI(),
		GetFromIndex(index),
		GetIDs(feedIDs...),
	)

	res, err := req.Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrSearchFailed, err)
	}

	var feeds []*models.APIFeed

	for _, doc := range res.Docs {
		switch obj := doc.(type) {
		case types.MultiGetError:
			c.Logger.Warn("Problem getting document", slog.Any("error", obj))
		case *types.GetResult:
			feed, err := ExtractSource[*models.APIFeed](obj.Source_)
			if err != nil {
				c.Logger.Warn("Could not unmarshal item source.", slog.Any("error", err))
				continue
			}

			feeds = append(feeds, feed)
		}
	}

	return feeds, nil
}

// GetFeedByID fetches a single feed by its ID.
func (c *Client) GetFeedByID(ctx context.Context, feedID models.FeedID) (*models.APIFeed, error) {
	feeds, err := c.GetFeedsByID(ctx, feedID)
	if err != nil {
		return nil, errors.Join(ErrReqFailed, err)
	}

	if len(feeds) == 0 {
		return nil, ErrNotFound
	}

	return feeds[0], nil
}

// ItemsSearch performs a search query on feed items with the given query
// options. It returns the raw search response.
func ItemsSearch(ctx context.Context, api *typedapi.API, query query.Option, filters api.Filters) (*search.Response, error) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	sortValues, err := decodePagination(filters.GetPagination())
	if err != nil {
		return nil, errors.Join(ErrSearchFailed, err)
	}

	resp, err := NewSearchRequest(api,
		WithSearchIndex(index),
		WithSearchQueryOptions(query),
		WithSearchAfter(sortValues),
		WithSearchSize(filters.GetCount()),
		WithSortOptions(setItemSort(filters.GetSort())),
	).Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrSearchFailed, err)
	}

	return resp, nil
}

// ItemsAggregation performs a search aggregation (i.e., only aggregations returned) on feed items with the given query
// options. It returns the raw search response.
func ItemsAggregation(ctx context.Context, api *typedapi.API, query query.Option, aggregation Aggregation) (*search.Response, error) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	req := NewSearchRequest(api,
		WithSearchIndex(index),
		WithSearchQueryOptions(query),
		WithSearchSize(0),
		WithAggregations(aggregation),
		WithSortOptions(defaultItemSort()),
	)

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrUserActionFailed, err)
	}

	return resp, nil
}

// ItemsCount performs a count query on feed items with the given query
// options. It returns the raw count response.
func ItemsCount(ctx context.Context, api *typedapi.API, query query.Option) (*count.Response, error) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrCountFailed, ErrFetchCtx)
	}

	req := NewCountRequest(api,
		WithCountIndex(index),
		WithCountQueryOptions(query),
	)

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrCountFailed, err)
	}

	return resp, nil
}

// defaultFeedSort sorts Feeds by updated or published date descending.
func defaultFeedSort() map[string]types.FieldSort {
	return map[string]types.FieldSort{
		"created_at": {Order: &sortorder.Desc, Format: &defaultDatetimeFormat},
		"feed_id":    {Order: &sortorder.Desc},
	}
}

// defaultItemSort sorts Items by updated or published date descending.
func defaultItemSort() map[string]types.FieldSort {
	return map[string]types.FieldSort{
		"@timestamp": {Order: &sortorder.Desc, Format: &defaultDatetimeFormat},
		"item_id":    {Order: &sortorder.Desc},
	}
}

func setFeedSort(sort api.Sort) map[string]types.FieldSort {
	sortOptions := make(map[string]types.FieldSort)
	sortOptions["feed_id"] = types.FieldSort{Order: &sortorder.Desc}

	var (
		sortField string
		sortOrder sortorder.SortOrder
	)

	switch sort.SortBy {
	case api.LastUpdated:
		sortField = "created_at"
		switch sort.SortOrder {
		case api.SortAsc:
			sortOrder = sortorder.Asc
		case api.SortDesc:
			sortOrder = sortorder.Desc
		}
	default:
		return defaultFeedSort()
	}

	sortOptions[sortField] = types.FieldSort{Order: &sortOrder, Format: &defaultDatetimeFormat}

	return sortOptions
}

func setItemSort(sort api.Sort) map[string]types.FieldSort {
	sortOptions := make(map[string]types.FieldSort)
	sortOptions["item_id"] = types.FieldSort{Order: &sortorder.Desc}

	var (
		sortField string
		sortOrder sortorder.SortOrder
	)

	switch sort.SortBy {
	case api.LastUpdated:
		sortField = "@timestamp"
		switch sort.SortOrder {
		case api.SortAsc:
			sortOrder = sortorder.Asc
		case api.SortDesc:
			sortOrder = sortorder.Desc
		}
	default:
		return defaultItemSort()
	}

	sortOptions[sortField] = types.FieldSort{Order: &sortOrder, Format: &defaultDatetimeFormat}

	return sortOptions
}

func AddFeedByURL(ctx context.Context, api *typedapi.API, url models.URL) (models.FeedID, error) {
	feed, err := models.NewFeedFromURL(ctx, url)
	if err != nil {
		return "", err
	}

	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return "", ErrFetchCtx
	}

	resp, err := NewDocCreateRequest(api,
		index,
		feed.ID,
		feed,
		refresh.True).
		Do(ctx)
	if err != nil {
		return "", errors.Join(ErrAddFailed, err)
	}

	logging.FromContext(ctx).Debug("Added feed.",
		slog.String("result", resp.Result.String()),
		slog.Int64("version", resp.Version_))

	return feed.ID, nil
}

func FindOrAddFeed(ctx context.Context, api *typedapi.API, url models.URL) (models.FeedID, error) {
	var feedID models.FeedID
	// Find any existing feed with the given subscription URL.
	feed, err := GetFeedByURL(ctx, api, url)
	if err != nil && !errors.Is(err, models.ErrNoFeed) {
		return "", errors.Join(models.ErrBackend, err)
	}
	// If there is no existing feed, create a new feed.
	if errors.Is(err, models.ErrNoFeed) {
		feedID, err = AddFeedByURL(ctx, api, url)
		if err != nil {
			return "", errors.Join(models.ErrBackend, err)
		}
	} else {
		feedID = feed.ID
	}

	return feedID, nil
}
