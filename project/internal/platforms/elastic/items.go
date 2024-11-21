// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/sortorder"

	"github.com/joshuar/go-feed-me/internal/models"
)

var ErrNoFeedID = errors.New("no feed ID provided")

func queryItemsByFeedID(id string) *types.Query {
	return &types.Query{
		Term: map[string]types.TermQuery{
			"feed_id": {Value: id},
		},
	}
}

func sortFeedItemsByTimestamp() types.SortOptions {
	return types.SortOptions{
		SortOptions: map[string]types.FieldSort{
			"@timestamp": {
				Order: &sortorder.Desc,
			},
		},
	}
}

func getSelectedFields(field ...string) []types.FieldAndFormat {
	fields := make([]types.FieldAndFormat, len(field))
	for i, name := range field {
		fields[i] = types.FieldAndFormat{Field: name}
	}

	return fields
}

func (c *Client) GetFeedItemsSummary(ctx context.Context, feedID string) ([]models.APIFeedItemSummary, error) {
	if feedID == "" {
		return nil, ErrNoFeedID
	}

	req := c.API.Search().
		Index("feeditems-*").
		Fields(getSelectedFields("@timestamp", "title", "description", "item_id", "image")...).
		Query(queryItemsByFeedID(feedID)).
		Sort(sortFeedItemsByTimestamp())
	res, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get feed item summaries: %w", err)
	}

	var items []models.APIFeedItemSummary

	for _, hit := range res.Hits.Hits {
		var item models.APIFeedItemSummary

		if err := json.Unmarshal([]byte(hit.Source_), &item); err != nil {
			c.logger.Warn("Could not unmarshal item source.", slog.Any("error", err))
			continue
		}
		// spew.Dump(hit.Source_)
		// godump.Dump(item)
		items = append(items, item)
	}

	return items, nil
}
