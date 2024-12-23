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
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/schema"
)

var ErrNoFeedID = errors.New("no feed ID provided")

var defaultItemFields = []string{"publishedParsed", "updatedParsed", "title", "description", "item_id", "image"}

func (c *Client) GetItems(ctx context.Context, filters models.APISearchFilters) ([]models.APIItem, []byte, error) {
	if len(filters.FeedIDs) == 0 {
		return nil, nil, ErrNoFeedID
	}

	req := c.NewSearchRequest(
		WithIndexPattern[*search.Search](schema.FeedItemsSchemaPrefix+"-*"),
		WithFields(defaultItemFields...),
		WithQueryOptions(QueryByFeedIDs(filters.FeedIDs...)),
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

	// Get the sort value(s) of the last hit.
	data, err := json.Marshal(res.Hits.Hits[len(res.Hits.Hits)-1].Sort)
	if err != nil {
		c.logger.Warn("Cannot marshal sort value.", slog.Any("error", err))
	}

	return items, data, nil
}

func (c *Client) AddFeedItems(_ context.Context, items ...models.Item) error {
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
		WithQueryOptions(
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
