// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/schema"
)

var defaultFeedFields = []string{"publishedParsed", "updatedParsed", "feed_id", "title", "description", "feedLink", "image", "categories", "authors"}

// GetFeeds returns the feeds with the given feed IDs. If no feed IDs are given,
// it returns all feeds.
func (c *Client) GetFeeds(ctx context.Context, feedIDs ...string) ([]models.APIFeed, error) {
	var queryOpt QueryOption
	if len(feedIDs) > 0 {
		queryOpt = QueryByFeedIDs(feedIDs...)
	} else {
		queryOpt = QueryMatchAll()
	}

	req := c.NewSearchRequest(
		IndexPattern(schema.FeedSchemaPrefix+"-*"),
		WithFields(defaultFeedFields...),
		WithQueryOptions(queryOpt),
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

func (c *Client) GetNewFeedsSince(ctx context.Context, since time.Time) ([]models.APIFeed, error) {
	req := c.NewSearchRequest(
		IndexPattern(schema.FeedSchemaPrefix+"-*"),
		WithQueryOptions(QuerySince("@timestamp", since)),
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

func (c *Client) GetFeedByURL(ctx context.Context, url string) (models.APIFeed, error) {
	var feed models.APIFeed

	req := c.NewSearchRequest(
		IndexPattern(schema.FeedSchemaPrefix+"-*"),
		WithFields(defaultFeedFields...),
		WithQueryOptions(QueryByTerm("feedLink", url)),
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

func (c *Client) AddFeeds(ctx context.Context, feeds ...models.Feed) error {
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
