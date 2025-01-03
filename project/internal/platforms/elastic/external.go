// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/count"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/mget"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/schema"
	"github.com/joshuar/go-feed-me/internal/server/session"
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
	ErrNoFeedID  = errors.New("no feed ID provided")
	ErrNoUserCtx = errors.New("no valid user in context")
)

// getFeedsByID retrieves the specified feeds with an mget request.
func (c *Client) getFeedsByID(ctx context.Context, ids ...string) ([]models.APIFeed, error) {
	req := c.NewMGetRequest(
		WithIndexPattern[*mget.Mget]("feeds"),
		WithIDs(ids...),
		// WithStoredFields(defaultFeedFields...),
	)

	res, err := req.Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrSearchFailed, err)
	}

	var feeds []models.APIFeed

	for _, doc := range res.Docs {
		switch obj := doc.(type) {
		case types.MultiGetError:
			c.logger.Warn("Problem getting document", slog.Any("error", obj))
		case *types.GetResult:
			var feed models.APIFeed

			if err := json.Unmarshal(obj.Source_, &feed); err != nil {
				c.logger.Warn("Could not unmarshal item source.", slog.Any("error", err))
				continue
			}

			feeds = append(feeds, feed)
		}
	}

	return feeds, nil
}

// getAllFeeds retrieves all feeds by executing a search request with a
// match_all query.
//
//nolint:prealloc
func (c *Client) getAllFeeds(ctx context.Context) ([]models.APIFeed, error) {
	req := c.NewSearchRequest(
		WithIndexPattern[*search.Search](schema.FeedSchemaPrefix+"-*"),
		WithFields(defaultFeedFields...),
		WithSearchQueryOptions(QueryMatchAll()),
	)

	res, err := req.Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrSearchFailed, err)
	}

	var feeds []models.APIFeed

	for _, hit := range res.Hits.Hits {
		var feed models.APIFeed

		if err := json.Unmarshal(hit.Source_, &feed); err != nil {
			c.logger.Warn("Could not unmarshal item source.", slog.Any("error", err))
			continue
		}
		// spew.Dump(hit.Source_)
		// godump.Dump(item)
		feeds = append(feeds, feed)
	}

	return feeds, nil
}

// GetFeeds returns the feeds with the given feed IDs. If no feed IDs are given,
// it returns all feeds. This will either be an mget (specific feeds) or search
// (all feeds) request.
func (c *Client) GetFeeds(ctx context.Context, filters models.APISearchFilters) ([]models.APIFeed, error) {
	if len(filters.FeedIDs) > 0 {
		return c.getFeedsByID(ctx, filters.FeedIDs...)
	}

	return c.getAllFeeds(ctx)
}

//nolint:prealloc
func (c *Client) GetNewFeedsSince(ctx context.Context, since time.Time) ([]models.APIFeed, error) {
	req := c.NewSearchRequest(
		WithIndexPattern[*search.Search](schema.FeedSchemaPrefix+"-*"),
		WithSearchQueryOptions(QuerySince("@timestamp", since)),
	)

	res, err := req.Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrSearchFailed, err)
	}

	var feeds []models.APIFeed

	for _, hit := range res.Hits.Hits {
		var feed models.APIFeed

		if err := json.Unmarshal(hit.Source_, &feed); err != nil {
			c.logger.Warn("Could not unmarshal item source.", slog.Any("error", err))
			continue
		}
		// spew.Dump(hit.Source_)
		// godump.Dump(item)
		feeds = append(feeds, feed)
	}

	// godump.Dump(feeds)

	return feeds, nil
}

func (c *Client) GetFeedByURL(ctx context.Context, url string) (models.APIFeed, error) {
	var feed models.APIFeed

	req := c.NewSearchRequest(
		WithIndexPattern[*search.Search](schema.FeedSchemaPrefix+"-*"),
		WithFields(defaultFeedFields...),
		WithSearchQueryOptions(QueryByTerm("feedLink", url)),
	)

	res, err := req.Do(ctx)
	if err != nil {
		return feed, errors.Join(ErrSearchFailed, err)
	}

	// If there are no hits, just return an empty APIFeed object.
	if res.Hits.Total.Value == 0 {
		return feed, nil
	}

	if err := json.Unmarshal(res.Hits.Hits[0].Source_, &feed); err != nil {
		return feed, errors.Join(ErrSearchFailed, err)
	}

	return feed, nil
}

//nolint:prealloc
func (c *Client) AddFeeds(_ context.Context, feeds ...models.Feed) error {
	var docs []document

	for _, feed := range feeds {
		c.logger.Debug("Adding feed",
			slog.String("name", feed.Title),
			slog.String("item_id", feed.ID),
		)

		docs = append(docs, &feed)
	}

	c.bulkStream <- docs

	return nil
}

func (c *Client) CountUnread(ctx context.Context, feedIDs ...string) (int, error) {
	userID, err := session.UserID(ctx)
	if err != nil {
		return 0, errors.Join(ErrNoUserCtx, err)
	}

	readItems, err := c.getUserReadItems(ctx, userID, models.APISearchFilters{FeedIDs: feedIDs})
	if err != nil {
		return 0, errors.Join(ErrSearchFailed, err)
	}

	// Extract the read item IDs.
	itemIDs := models.ReadItemIDs(readItems)

	// Create the count query.
	countReq := c.NewCountRequest(
		WithIndexPattern[*count.Count](schema.FeedItemsSchemaPrefix+"-*"),
		WithCountQueryOptions(
			QueryBool(
				BoolFilter(QueryByFeedIDs(feedIDs...)),
				BoolMustNot(QueryByItemIDs(itemIDs...))),
		),
	)

	countRes, err := countReq.Do(ctx)
	if err != nil {
		return 0, errors.Join(ErrCountFailed, err)
	}

	return int(countRes.Count), nil
}

func (c *Client) GetItems(ctx context.Context, filters models.APISearchFilters) ([]models.APIItem, []byte, error) {
	userID, err := session.UserID(ctx)
	if err != nil {
		return nil, nil, errors.Join(ErrNoUserCtx, err)
	}

	// Get the user read items.
	readItems, err := c.getUserReadItems(ctx, userID, filters)
	if err != nil {
		c.logger.Warn("Could not get user read items.",
			slog.Any("error", err))
	}

	req := c.NewSearchRequest(
		WithIndexPattern[*search.Search](schema.FeedItemsSchemaPrefix+"-*"),
		WithFields(defaultItemFields...),
		WithSearchQueryOptions(
			QueryBool(
				BoolFilter(QueryByFeedIDs(filters.FeedIDs...)),
				BoolMustNot(QueryByItemIDs(models.ReadItemIDs(readItems)...)),
			),
		),
		WithSortOptions(SortTimestampDesc()),
		SearchSize(10),
	)

	if filters.Pagination != nil {
		var searchAfter []types.FieldValue
		if err := json.Unmarshal(filters.Pagination, &searchAfter); err != nil {
			c.logger.Warn("Could not unmarshal pagination data.", slog.Any("error", err))
		} else {
			req = SearchAfter(searchAfter)(req)
		}
	}

	res, err := req.Do(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get feed item summaries: %w", err)
	}

	var items []models.APIItem

	for _, hit := range res.Hits.Hits {
		var item models.APIItem

		if err := json.Unmarshal([]byte(hit.Source_), &item); err != nil {
			c.logger.Warn("Could not unmarshal item source.", slog.Any("error", err))
			continue
		}

		items = append(items, item)
	}

	spew.Dump(items)

	// Get the sort value(s) of the last hit.
	data, err := json.Marshal(res.Hits.Hits[len(res.Hits.Hits)-1].Sort)
	if err != nil {
		c.logger.Warn("Cannot marshal sort value.", slog.Any("error", err))
	}

	return items, data, nil
}

// AddItems will bulk index the given items to the Elasticsearch cache.
func (c *Client) AddItems(_ context.Context, items ...models.Item) error {
	docs := make([]document, len(items))

	for iter, item := range items {
		c.logger.Debug("Adding item",
			slog.String("name", item.Title),
			slog.String("item_id", item.ID),
			slog.String("feed_id", item.FeedID),
		)

		docs[iter] = &item
	}

	c.bulkStream <- docs

	return nil
}

// GetItem retrieves the specified item with the given id and from the given
// feed.
func (c *Client) GetItem(ctx context.Context, feedID, itemID string) (models.APIItem, error) {
	req := c.NewSearchRequest(
		WithIndexPattern[*search.Search](schema.FeedItemsSchemaPrefix+"-*"),
		WithFields(defaultItemFields...),
		WithSearchQueryOptions(
			QueryByFeedIDs(feedID),
			QueryByItemIDs(itemID)),
		WithSortOptions(SortTimestampDesc()),
	)

	res, err := req.Do(ctx)
	if err != nil {
		return models.APIItem{}, errors.Join(ErrSearchFailed, err)
	}

	var item models.APIItem

	for _, hit := range res.Hits.Hits {
		if err := json.Unmarshal(hit.Source_, &item); err != nil {
			c.logger.Warn("Could not unmarshal item source.", slog.Any("error", err))
			continue
		}

		break
	}

	return item, nil
}

// MarkItemsRead will bulk index the given items to the readitems index,
// effectively, marking them as read for a particular user.
func (c *Client) MarkItemsRead(ctx context.Context, items ...models.APIReadItem) error {
	userID, err := session.UserID(ctx)
	if err != nil {
		return errors.Join(ErrNoUserCtx, err)
	}

	docs := make([]document, len(items))

	for iter, item := range items {
		c.logger.Debug("Marking item read",
			slog.String("item_id", item.ItemID),
			slog.String("feed_id", item.FeedID),
		)

		docs[iter] = &models.ReadItem{
			Timestamp: time.Now(),
			ItemID:    item.ItemID,
			FeedID:    item.FeedID,
			UserID:    userID,
		}
	}

	c.bulkStream <- docs

	return nil
}

func (c *Client) MarkFeedsRead(ctx context.Context, feedIDs ...models.FeedID) error {
	return nil
}

func (c *Client) getUserReadItems(ctx context.Context, userID models.UserID, filters models.APISearchFilters) (models.ReadItems, error) {
	resp, err := c.NewSearchRequest(
		WithIndexPattern[*search.Search](schema.ReadItemsSchemaPrefix+"-*"),
		WithFields("user_id"),
		WithSearchQueryOptions(
			QueryBool(
				BoolFilter(
					QueryByTerm("user_id", userID),
					QueryByFeedIDs(filters.FeedIDs...)),
			),
		),
	).Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrSearchFailed, err)
	}

	items := unmarshalSource[models.ReadItem](ctx, resp.Hits.Hits)

	return items, nil
}

// unmarshalSource loops through the given hits array and extracts the `_source`
// field of each document as type `T`, returning the documents as `[]T`.
//
//nolint:prealloc
func unmarshalSource[T any](ctx context.Context, hits []types.Hit) []T {
	var items []T

	for _, hit := range hits {
		var item T

		if err := json.Unmarshal([]byte(hit.Source_), &item); err != nil {
			logging.FromContext(ctx).Warn("Could not unmarshal item source.",
				slog.Any("error", err))
			continue
		}

		items = append(items, item)
	}

	return items
}
