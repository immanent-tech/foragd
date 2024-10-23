// Copyright (C) 2024 Joshua Rich <joshua.rich@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package elastic

import (
	"context"
	"log/slog"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/joshuar/go-feed-me/platform/feeds"
)

func (c *Client) CacheFeedItems(stream chan feeds.FeedItem) {
	var items []feeds.FeedItem //nolint:prealloc

	for item := range stream {
		c.logger.Debug("Adding item", slog.String("name", item.Title))
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
				itemID := item.ID()
				op := types.NewCreateOperation()
				op.Id_ = &itemID

				if err := bulkOp.CreateOp(*op, item); err != nil {
					c.logger.Warn("Failed to create index operation for item.",
						slog.String("guid", item.GUID),
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
