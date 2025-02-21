// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/joshuar/go-feed-me/internal/models"
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

var (
	ErrNoFeedID      = errors.New("no feed ID provided")
	ErrExtractSource = errors.New("could not extract document _source")
	ErrAddFailed     = errors.New("adding items failed")
)

// GetNewFeedsSince retrieves a list of feeds that have been updated since the
// given time.
func (c *Client) GetNewFeedsSince(ctx context.Context, since time.Time) ([]models.APIFeed, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrNoIndexInCtx)
	}

	c.Logger.Debug("Finding new feeds.",
		slog.Time("since", since))

	var newFeeds []models.APIFeed

	searchSize := 100
	pagination := make([]types.FieldValue, 0)

	for {
		var (
			feeds    []models.APIFeed
			warnings error
		)

		resp, err := c.NewSearchRequest(
			WithSearchIndex(index),
			WithSearchQueryOptions(QuerySince("created_at", since)),
			WithSearchSize(searchSize),
			WithSearchAfter(pagination),
		).Do(ctx)
		if err != nil {
			return nil, errors.Join(ErrSearchFailed, err)
		}

		feeds, pagination, warnings = ExtractSourceFromHits[models.APIFeed](resp.Hits.Hits)
		if warnings != nil {
			c.Logger.Warn("Problems occurred while extracting source from docs.",
				slog.Any("warnings", err))
		}

		newFeeds = append(newFeeds, feeds...)

		// Stop if we are at the end of the results.
		if int(resp.Hits.Total.Value) < searchSize {
			break
		}
	}

	return newFeeds, nil
}

func (c *Client) GetFeedByURL(ctx context.Context, url string) (*models.APIFeed, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrNoIndexInCtx)
	}

	resp, err := c.NewSearchRequest(
		WithSearchIndex(index),
		WithFields(defaultFeedFields...),
		WithSearchQueryOptions(QueryByTerm("feedLink", url)),
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
func (c *Client) GetFeedsByURL(ctx context.Context, urls ...models.URL) ([]models.APIFeed, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrNoIndexInCtx)
	}

	feeds := make([]models.APIFeed, 0, len(urls))

	resp, err := c.NewSearchRequest(
		WithSearchIndex(index),
		WithFields("feed_id", "feedLink"),
		WithSearchQueryOptions(QueryByURLs("feedLink", urls...)),
		WithSearchSize(len(urls)),
	).Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrSearchFailed, err)
	}
	// Stop if there are no hits
	if len(resp.Hits.Hits) == 0 {
		return nil, nil
	}
	// Loop through this set of results.
	sources, _, warnings := ExtractSourceFromHits[models.APIFeed](resp.Hits.Hits)
	if warnings != nil {
		c.Logger.Warn("Problems occurred while extracting source from docs.",
			slog.Any("warnings", err))
	}

	feeds = append(feeds, sources...)

	return feeds, nil
}

// FeedsSearch searches the feeds index for feeds matching the relevant filters.
func (c *Client) FeedsSearch(ctx context.Context, filters models.APIFilters) ([]*models.APIFeed, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrNoIndexInCtx)
	}

	resp, err := c.NewSearchRequest(
		WithSearchIndex(index),
		WithSearchQueryOptions(
			QueryBool(
				// Match either the FeedID OR the Category.
				BoolShould(
					QueryByFeedIDs(filters.FeedIDs...),
					QueryByCategory(filters.Categories...),
				),
			),
		),
		WithSearchSize(filters.Count),
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
		c.Logger.Warn("Problems occurred while extracting source from docs.",
			slog.Any("warnings", err))
	}

	return sources, nil
}

func (c *Client) GetFeedCategories(ctx context.Context, feedIDs ...models.FeedID) (*TermsAggregationResults, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrNoIndexInCtx)
	}

	// Find all feeds with the given IDs, build a terms aggregation on the
	// categories field.
	req := c.NewSearchRequest(
		WithSearchIndex(index),
		WithSearchQueryOptions(
			QueryByFeedIDs(feedIDs...),
		),
		WithSortOptions(SortTimestampDesc()),
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
		return nil, errors.Join(ErrSearchFailed, ErrNoIndexInCtx)
	}

	// Get the feed details.
	req := c.NewMGetRequest(
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

func (c *Client) AddFeeds(ctx context.Context, feeds ...models.Feed) error {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return errors.Join(ErrAddFailed, ErrNoIndexInCtx)
	}

	docs := make([]*BulkOperation, len(feeds))

	for iter, feed := range feeds {
		feed.Items = nil // don't index items in feed.
		c.Logger.Debug("Adding feed",
			slog.String("name", feed.Title),
			slog.String("item_id", feed.ID),
		)

		docs[iter] = NewBulkOperation(&feed,
			SetDocID(feed.ID),
			ToIndex(index),
		)
	}

	c.bulkStream <- docs

	return nil
}

// AddItems will bulk index the given items to the Elasticsearch cache.
func (c *Client) AddItems(ctx context.Context, items ...models.Item) error {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return errors.Join(ErrAddFailed, ErrNoIndexInCtx)
	}

	docs := make([]*BulkOperation, len(items))

	for iter, item := range items {
		c.Logger.Debug("Adding item",
			slog.String("name", item.Title),
			slog.String("item_id", item.ID),
			slog.String("feed_id", item.FeedID),
		)

		docs[iter] = NewBulkOperation(&item,
			SetDocID(item.ID),
			ToIndex(index),
		)
	}

	c.bulkStream <- docs

	return nil
}

func (c *Client) ItemsSearch(ctx context.Context, query QueryOption, size models.Count) (*search.Response, error) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrNoIndexInCtx)
	}

	req := c.NewSearchRequest(
		WithSearchIndex(index),
		WithSearchQueryOptions(query),
		WithSortOptions(SortTimestampDesc()),
		WithSearchSize(size),
	)

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrUserActionFailed, err)
	}

	return resp, nil
}

func (c *Client) ItemsAggregation(ctx context.Context, query QueryOption, aggregation Aggregation) (*search.Response, error) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrNoIndexInCtx)
	}

	req := c.NewSearchRequest(
		WithSearchIndex(index),
		WithSearchQueryOptions(query),
		WithSortOptions(SortTimestampDesc()),
		WithSearchSize(0),
		WithAggregations(aggregation),
	)

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrUserActionFailed, err)
	}

	return resp, nil
}
