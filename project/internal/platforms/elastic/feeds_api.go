// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/mget"
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

	resp, err := c.NewSearchRequest(
		WithIndex[*search.Search](index),
		WithSearchQueryOptions(QuerySince("@timestamp", since)),
	).Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrSearchFailed, err)
	}

	feeds := ExtractSources[models.APIFeed](ctx, resp.Hits.Hits)

	return feeds, nil
}

func (c *Client) GetFeedByURL(ctx context.Context, url string) (models.APIFeed, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return models.APIFeed{}, errors.Join(ErrSearchFailed, ErrNoIndexInCtx)
	}

	resp, err := c.NewSearchRequest(
		WithIndex[*search.Search](index),
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

// GetFeedsByID fetches a list of feeds by their FeedID.
func (c *Client) GetFeedsByID(ctx context.Context, feedIDs ...models.FeedID) ([]*models.APIFeed, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrNoIndexInCtx)
	}

	// Get the feed details.
	req := c.NewMGetRequest(
		WithIndex[*mget.Mget](index),
		WithIDs[*mget.Mget](feedIDs...),
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

func (c *Client) AddFeeds(ctx context.Context, feeds ...models.Feed) error {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return errors.Join(ErrAddFailed, ErrNoIndexInCtx)
	}

	docs := make([]BulkOperation, len(feeds))

	for iter, feed := range feeds {
		feed.Items = nil // don't index items in feed.
		c.Logger.Debug("Adding feed",
			slog.String("name", feed.Title),
			slog.String("item_id", feed.ID),
		)

		docs[iter] = NewBulkOperation(&feed,
			WithDocID[BulkOperation](feed.ID),
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

	docs := make([]BulkOperation, len(items))

	for iter, item := range items {
		c.Logger.Debug("Adding item",
			slog.String("name", item.Title),
			slog.String("item_id", item.ID),
			slog.String("feed_id", item.FeedID),
		)

		docs[iter] = NewBulkOperation(&item,
			WithDocID[BulkOperation](item.ID),
			ToIndex(index),
		)
	}

	c.bulkStream <- docs

	return nil
}
