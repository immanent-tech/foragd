// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"log/slog"

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/bulk"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/schema"
)

type BulkRequest struct {
	*bulk.Bulk
}

// document represents a single record in elasticsearch.
type document interface {
	DocumentType() models.DocumentType
	DocumentID() *string
}

// BulkOption sets an option on a bulk request.
type BulkOption func(*bulk.Bulk) *bulk.Bulk

// WithPipeline defines the ingest pipeline to use on each document.
func WithPipeline(pipeline string) BulkOption {
	return func(b *bulk.Bulk) *bulk.Bulk {
		b = b.Pipeline(pipeline)
		return b
	}
}

// WithIndex defines the index on which the request will operate.
func WithIndex(index string) BulkOption {
	return func(b *bulk.Bulk) *bulk.Bulk {
		b = b.Index(index)
		return b
	}
}

// CreateDocs will add create operations for the given docs to the bulk request.
func CreateDocs(documents ...document) BulkOption {
	return func(bulkReq *bulk.Bulk) *bulk.Bulk {
		for _, doc := range documents {
			var index string

			switch doc.DocumentType() {
			case models.TypeFeed:
				index = "feeds-test"
			case models.TypeItem:
				index = "feeditems-test"
			case models.TypeUser:
				index = "users-test"
			}

			if err := bulkReq.CreateOp(types.CreateOperation{Index_: &index, Id_: doc.DocumentID()}, doc); err != nil {
				slog.Warn("Could not generate a create op for document.",
					slog.Any("type", doc.DocumentType()),
					slog.String("id", *doc.DocumentID()),
					slog.Any("error", err))

				continue
			}
		}

		return bulkReq
	}
}

// NewSearchRequest creates a new search object with the given options.
func (c *Client) NewBulkRequest(options ...BulkOption) *BulkRequest {
	req := c.API.Bulk()

	for _, option := range options {
		req = option(req)
	}

	return &BulkRequest{Bulk: req}
}

// bulkStreamWorker runs in a goroutine and listens for documents to bulk index
// into Elasticsearch.
func (c *Client) bulkStreamWorker(ctx context.Context) {
	c.Logger.Debug("Bulk indexer ready...")

	for {
		select {
		case <-ctx.Done():
			return
		case items := <-c.bulkStream:
			if len(items) == 0 {
				continue
			}

			go func() {
				// Create a new bulk request.
				resp, err := c.NewBulkRequest(
					WithPipeline(schema.IngestPipelineID),
					CreateDocs(items...),
				).Do(ctx)
				// Handle response.
				switch {
				case err != nil:
					c.Logger.Error("Bulk index failed.",
						slog.Any("error", err))
				case resp.Errors:
					c.Logger.Warn("Bulk index completed with some errors.",
						slog.Any("errors", resp.Items),
					)
				}

				c.Logger.Debug("Bulk indexed items.",
					slog.Int("count", len(resp.Items)),
					slog.Int("took", int(resp.Took)),
				)
			}()
		}
	}
}
