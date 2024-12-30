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

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/count"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/mget"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

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
		return 0, fmt.Errorf("unable to get unread items: %w", err)
	}
	// Find all read items for this user and the given feeds.
	readItemsReq := c.NewSearchRequest(
		WithIndexPattern[*search.Search](schema.ReadItemsSchemaPrefix+"-*"),
		WithFields("user_id"),
		WithSearchQueryOptions(
			QueryBool(
				BoolFilter(
					QueryByTerm("user_id", userID),
					QueryByFeedIDs(feedIDs...)),
			),
		),
	)

	readItemsRes, err := readItemsReq.Do(ctx)
	if err != nil {
		return 0, errors.Join(ErrSearchFailed, err)
	}

	// Extract the read item IDs.
	var itemIDs []string

	for _, hit := range readItemsRes.Hits.Hits {
		itemIDs = append(itemIDs, string(hit.Fields["user_id"]))
	}

	var queryOpts Option[*count.Count]

	// Create the query options depending on whether there are any unread items
	// to filter.
	if len(itemIDs) > 0 {
		queryOpts = WithCountQueryOptions(
			QueryBool(
				BoolFilter(QueryByFeedIDs(feedIDs...)),
				BoolMustNot(QueryByItemIDs(itemIDs...))),
		)
	} else {
		queryOpts = WithCountQueryOptions(
			QueryBool(BoolFilter(QueryByFeedIDs(feedIDs...))))
	}

	// Create the count query.
	countReq := c.NewCountRequest(
		WithIndexPattern[*count.Count](schema.FeedItemsSchemaPrefix+"-*"),
		queryOpts,
	)

	countRes, err := countReq.Do(ctx)
	if err != nil {
		return 0, errors.Join(ErrCountFailed, err)
	}

	return int(countRes.Count), nil
}
