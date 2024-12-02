// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/bulk"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/joshuar/go-feed-me/internal/platforms/elastic/schema"
)

type BulkRequest struct {
	*bulk.Bulk
}

// document represents a single record in elasticsearch.
type document interface {
	DocID() string
}

func (b *BulkRequest) generateItemCreateOps(documents ...document) error {
	var errs error

	for _, doc := range documents {
		err := b.CreateOp(NewCreateOp(doc.DocID()), doc)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("could not generate create op for %s: %w", doc.DocID(), err))
			continue
		}
	}

	return errs
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

// NewSearchRequest creates a new search object with the given options.
func (c *Client) NewBulkRequest(options ...BulkOption) *BulkRequest {
	req := c.API.Bulk()

	for _, option := range options {
		req = option(req)
	}

	return &BulkRequest{Bulk: req}
}

func NewCreateOp(id string) types.CreateOperation {
	op := types.NewCreateOperation()
	op.Id_ = &id

	return *op
}

func (c *Client) bulkStreamWorker(ctx context.Context) {
	c.logger.Debug("Bulk indexer ready...")

	for {
		select {
		case <-ctx.Done():
			return
		case items := <-c.bulkStream:
			if len(items) == 0 {
				continue
			}

			go func() {
				var req *BulkRequest
				// Create a new bulk request.
				switch {
				case strings.HasPrefix(items[0].DocID(), "item"):
					req = c.NewBulkRequest(
						WithIndex("feeditems-test"),
						WithPipeline(schema.IngestPipelineID),
					)
				case strings.HasPrefix(items[0].DocID(), "feed"):
					req = c.NewBulkRequest(
						WithIndex("feeds-test"),
						WithPipeline(schema.IngestPipelineID),
					)
				}
				// Add all the items to the bulk request.
				if err := req.generateItemCreateOps(items...); err != nil {
					c.logger.Warn("Problems encountered when generating create operations.",
						slog.Any("error", err))
				}
				// Execute the bulk request.
				resp, err := req.Do(ctx)
				// Handle response.
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
