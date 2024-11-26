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

func queryItemsByFeedIDs(feedIDs ...string) *types.Query {
	return &types.Query{
		Terms: &types.TermsQuery{
			TermsQuery: map[string]types.TermsQueryField{
				"feed_id": feedIDs,
			},
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

func (c *Client) GetFeedItemsSummary(ctx context.Context, feedIDs ...string) ([]models.APIItem, error) {
	if feedIDs == nil {
		return nil, ErrNoFeedID
	}

	req := c.API.Search().
		Index("feeditems-*").
		Fields(getSelectedFields("@timestamp", "title", "description", "item_id", "image")...).
		Query(queryItemsByFeedIDs(feedIDs...)).
		Sort(sortFeedItemsByTimestamp()).Size(20)
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
		// spew.Dump(hit.Source_)
		// godump.Dump(item)
		items = append(items, item)
	}

	return items, nil
}
