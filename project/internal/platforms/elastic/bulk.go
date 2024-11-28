// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"log/slog"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/joshuar/go-feed-me/internal/platforms/elastic/schema"
)

// func (c *Client) CacheFeedItems(stream chan models.Item) {
// 	var items []models.Item //nolint:prealloc

// 	for item := range stream {
// 		c.logger.Debug("Adding item",
// 			slog.String("name", item.Title),
// 			slog.String("item_id", item.ID),
// 			slog.String("feed_id", item.FeedID),
// 		)

// 		items = append(items, item)
// 	}

// 	c.feedItemsBulkStream <- items
// }

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
				itemID := item.ID
				op := types.NewCreateOperation()
				op.Id_ = &itemID

				slog.Info("indexing item", slog.Any("item", item))

				if err := bulkOp.CreateOp(*op, item); err != nil {
					c.logger.Warn("Failed to create index operation for item.",
						slog.String("item_id", item.ID),
						slog.String("feed_id", item.FeedID),
						slog.Any("error", err))
				}
			}

			go func() {
				resp, err := bulkOp.Index("feeditems-test").Pipeline(schema.IngestPipelineID).Do(ctx)

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

func (c *Client) bulkIndexFeedsWorker(ctx context.Context) {
	c.logger.Debug("Bulk indexer ready...")

	for {
		select {
		case <-ctx.Done():
			return
		case feed := <-c.feedsBulkStream:

			bulkOp := c.API.Bulk()

			// data, _ := json.Marshal(op)
			// fmt.Fprintf(os.Stdout, "%s\n\n", data)
			feedID := feed.ID
			op := types.NewCreateOperation()
			op.Id_ = &feedID

			slog.Info("indexing feed", slog.Any("feed", feed))

			if err := bulkOp.CreateOp(*op, feed); err != nil {
				c.logger.Warn("Failed to create index operation for item.",
					slog.String("item_id", feed.ID),
					slog.Any("error", err))
			}

			go func() {
				resp, err := bulkOp.Index("feeds-test").Pipeline(schema.IngestPipelineID).Do(ctx)

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
