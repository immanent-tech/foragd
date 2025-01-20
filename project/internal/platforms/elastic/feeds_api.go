// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"

	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/schema"
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
)

// // GetFeeds returns the feeds with the given feed IDs. If no feed IDs are given,
// // it returns all feeds. This will either be an mget (specific feeds) or search
// // (all feeds) request.
// func (c *Client) GetFeeds(ctx context.Context, filters models.APISearchFilters) ([]models.APIFeed, error) {
// 	if len(filters.FeedIDs) > 0 {
// 		return c.getFeedsByID(ctx, filters.FeedIDs...)
// 	}

// 	return c.getAllFeeds(ctx)
// }

// GetNewFeedsSince retrieves a list of feeds that have been updated since the
// given time.
func (c *Client) GetNewFeedsSince(ctx context.Context, since time.Time) ([]models.APIFeed, error) {
	resp, err := c.NewSearchRequest(
		WithIndexPattern[*search.Search](schema.FeedsSchemaPrefix+"-*"),
		WithSearchQueryOptions(QuerySince("@timestamp", since)),
	).Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrSearchFailed, err)
	}

	feeds := ExtractSources[models.APIFeed](ctx, resp.Hits.Hits)

	return feeds, nil
}

func (c *Client) GetFeedByURL(ctx context.Context, url string) (models.APIFeed, error) {
	resp, err := c.NewSearchRequest(
		WithIndexPattern[*search.Search](schema.FeedsSchemaPrefix+"-*"),
		WithFields(defaultFeedFields...),
		WithSearchQueryOptions(QueryByTerm("feedLink", url)),
	).Do(ctx)
	if err != nil {
		return models.APIFeed{}, errors.Join(ErrSearchFailed, err)
	}

	// If there are no hits, just return an empty APIFeed object.
	if resp.Hits.Total.Value == 0 {
		return models.APIFeed{}, nil
	}

	feed, err := ExtractSource[models.APIFeed](resp.Hits.Hits[0].Source_)
	if err != nil {
		return feed, errors.Join(ErrSearchFailed, err)
	}

	return feed, nil
}

func (c *Client) AddFeeds(_ context.Context, feeds ...models.Feed) error {
	docs := make([]BulkOperation, len(feeds))

	for iter, feed := range feeds {
		c.Logger.Debug("Adding feed",
			slog.String("name", feed.Title),
			slog.String("item_id", feed.ID),
		)

		docs[iter] = NewBulkOperation(&feed,
			WithDocID[BulkOperation](feed.ID),
			ToIndex(schema.FeedsSchemaPrefix+"-test"),
		)
	}

	c.bulkStream <- docs

	return nil
}

// AddItems will bulk index the given items to the Elasticsearch cache.
func (c *Client) AddItems(_ context.Context, items ...models.Item) error {
	docs := make([]BulkOperation, len(items))

	for iter, item := range items {
		c.Logger.Debug("Adding item",
			slog.String("name", item.Title),
			slog.String("item_id", item.ID),
			slog.String("feed_id", item.FeedID),
		)

		docs[iter] = NewBulkOperation(&item,
			WithDocID[BulkOperation](item.ID),
			ToIndex(schema.FeedItemsSchemaPrefix+"-test"),
		)
	}

	c.bulkStream <- docs

	return nil
}
