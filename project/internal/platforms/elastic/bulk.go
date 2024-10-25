// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"log/slog"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/joshuar/go-feed-me/internal/models"
)

func (c *Client) CacheFeedItems(stream chan models.FeedItem) {
	var items []models.FeedItem //nolint:prealloc

	for item := range stream {
		c.logger.Debug("Adding item",
			slog.String("name", item.Title),
			slog.String("item_id", item.ItemID),
			slog.String("feed_id", item.FeedID),
		)

		items = append(items, item)
	}

	c.feedItemsBulkStream <- items
}

func (c *Client) bulkIndexFeedItemsWorker(ctx context.Context) {
	c.logger.Debug("Bulk indexer ready...")

	for {
		select {
		case <-ctx.Done():
			return
		case items := <-c.feedItemsBulkStream:
			if len(items) == 0 {
				continue
			}

			bulkOp := c.API.Bulk()

			for _, item := range items {
				// data, _ := json.Marshal(op)
				// fmt.Fprintf(os.Stdout, "%s\n\n", data)
				itemID := item.ItemID
				op := types.NewCreateOperation()
				op.Id_ = &itemID

				if err := bulkOp.CreateOp(*op, item); err != nil {
					c.logger.Warn("Failed to create index operation for item.",
						slog.String("item_id", item.ItemID),
						slog.String("feed_id", item.FeedID),
						slog.Any("error", err))
				}
			}

			go func() {
				resp, err := bulkOp.Index("feeditems-test").Pipeline("feeditems").Do(ctx)

				switch {
				case err != nil:
					c.logger.Error("Bulk index failed.",
						slog.Any("error", err))
				case resp.Errors:
					c.logger.Info("Bulk index completed with some errors.",
						slog.Any("errors", resp.Items),
					)
				}
			}()
		}
	}
}
