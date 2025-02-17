// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/bulk"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

const (
	BulkCreate BulkOpType = iota
	BulkIndex
	BulkDelete
	BulkUpdate
)

// BulkOpType represents the type of bulk operation to perform.
type BulkOpType int

// BulkOpOption is a functional option for a bulk operation.
type BulkOpOption Option[*BulkOperation]

// BulkOption is a functional option for a bulk request.
type BulkOption Option[*bulk.Bulk]

// BulkRequest represents a bulk request.
type BulkRequest struct {
	*bulk.Bulk
	logger *slog.Logger
}

// WithPipeline defines the ingest pipeline to use on each document.
func WithPipeline(pipeline string) BulkOption {
	return func(b *bulk.Bulk) {
		if pipeline != "" {
			b = b.Pipeline(pipeline)
		}
	}
}

// WithIndex defines the index on which the request will operate.
func WithOverallIndex(index string) BulkOption {
	return func(b *bulk.Bulk) {
		b = b.Index(index)
	}
}

// AddOperations adds document operations to the bulk request.
func (r *BulkRequest) AddOperations(operations ...*BulkOperation) *BulkRequest {
	for _, operation := range operations {
		var err error

		switch operation.opType {
		case BulkCreate:
			if operation.GetDocID() != "" {
				err = r.CreateOp(types.CreateOperation{Index_: &operation.index, Id_: &operation.id}, operation.document)
			} else {
				err = r.CreateOp(types.CreateOperation{Index_: &operation.index}, operation.document)
			}
		case BulkUpdate:
			if operation.GetDocID() == "" {
				err = fmt.Errorf("id is required for update operation")
			} else {
				err = r.UpdateOp(types.UpdateOperation{Index_: &operation.index, Id_: &operation.id}, operation.document, types.NewUpdateAction())
			}
		}

		if err != nil {
			r.logger.Warn("Could not process bulk operation.",
				slog.Any("error", err))
		}
	}

	return r
}

// NewBulkRequest creates a new bulk requesst object with the given options.
// After creation, document operations can be added with the AddOperations method.
func (c *Client) NewBulkRequest(options ...BulkOption) *BulkRequest {
	req := c.API.Bulk()

	for _, option := range options {
		option(req)
	}

	return &BulkRequest{
		Bulk:   req,
		logger: c.Logger.WithGroup("bulk"),
	}
}

// BulkOperation represents an individual document's bulk operation.
type BulkOperation struct {
	document any
	index    string
	opType   BulkOpType
	docIDOption
}

func SetDocID(id string) BulkOpOption {
	return func(operation *BulkOperation) {
		operation.SetDocID(id)
	}
}

// AsOperationType specifies the type of bulk operation to perform. If this option is
// not specified, the operation will default to a create operation.
func AsOperationType(opType BulkOpType) BulkOpOption {
	return func(operation *BulkOperation) {
		operation.opType = opType
	}
}

// ToIndex sets the index containing the document.
func ToIndex(index string) BulkOpOption {
	return func(operation *BulkOperation) {
		operation.index = index
	}
}

// NewBulkOperation creates a new bulk operation for a document with the given options.
func NewBulkOperation(doc any, options ...BulkOpOption) *BulkOperation {
	operation := &BulkOperation{
		document: doc,
	}

	for _, option := range options {
		option(operation)
	}

	return operation
}

// bulkStreamWorker runs in a goroutine and listens for documents to bulk index
// into Elasticsearch.
func (c *Client) bulkStreamWorker(ctx context.Context) {
	pipeline := IngestPipelineFromCtx(ctx)

	c.Logger.Debug("Bulk indexer ready...")

	for {
		select {
		case <-ctx.Done():
			return
		case operations := <-c.bulkStream:
			if len(operations) == 0 {
				continue
			}

			go func() {
				// Create a new bulk request.
				resp, err := c.NewBulkRequest(
					WithPipeline(pipeline),
				).
					AddOperations(operations...).
					Do(ctx)
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
