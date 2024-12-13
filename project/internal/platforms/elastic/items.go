// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"

	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/schema"
)

var ErrNoFeedID = errors.New("no feed ID provided")

var defaultItemFields = []string{"publishedParsed", "updatedParsed", "title", "description", "item_id", "image"}

func (c *Client) GetFeedItems(ctx context.Context, feedIDs ...string) ([]models.APIItem, error) {
	if feedIDs == nil {
		return nil, ErrNoFeedID
	}

	req := c.NewSearchRequest(
		WithIndexPattern[*search.Search](schema.FeedItemsSchemaPrefix+"-*"),
		WithFields(defaultItemFields...),
		WithQueryOptions(QueryByFeedIDs(feedIDs...)),
		WithSortOptions(SortTimestampDesc()),
	)

	res, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get feed item summaries: %w", err)
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

	return items, nil
}

func (c *Client) AddFeedItems(ctx context.Context, items ...models.Item) error {
	var docs []document

	for _, item := range items {
		c.logger.Debug("Adding item",
			slog.String("name", item.Title),
			slog.String("item_id", item.ID),
			slog.String("feed_id", item.FeedID),
		)

		docs = append(docs, &item)
	}

	c.bulkStream <- docs

	return nil
}
