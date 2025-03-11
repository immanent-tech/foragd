// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package bulk

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/elastic/go-elasticsearch/v8/typedapi"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/bulk"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

var ErrAddOp = errors.New("could not add operation to bulk request")

const (
	BulkCreate BulkOpType = iota
	BulkIndex
	BulkDelete
	BulkUpdate
)

type Client interface {
	GetAPI() *typedapi.API
	Log() *slog.Logger
}

// BulkOpType represents the type of bulk operation to perform.
type BulkOpType int

// BulkOpOption is a functional option for a bulk operation.
type BulkOpOption func(*BulkOperation)

// BulkOption is a functional option for a bulk request.
type BulkOption func(*BulkRequest)

// BulkRequest represents a bulk request.
type BulkRequest struct {
	*bulk.Bulk
}

// WithPipeline defines the ingest pipeline to use on each document.
func WithPipeline(pipeline string) BulkOption {
	return func(b *BulkRequest) {
		if pipeline != "" {
			b.Pipeline(pipeline)
		}
	}
}

// WithIndex defines the index on which the request will operate.
func WithOverallIndex(index string) BulkOption {
	return func(b *BulkRequest) {
		b.Index(index)
	}
}

// AddOperations adds document operations to the bulk request.
func (r *BulkRequest) AddOperation(operation BulkOperation) error {
	var err error

	switch operation.opType {
	case BulkCreate:
		if operation.id != "" {
			err = r.CreateOp(types.CreateOperation{Index_: &operation.index, Id_: &operation.id}, operation.document)
		} else {
			err = r.CreateOp(types.CreateOperation{Index_: &operation.index}, operation.document)
		}
	case BulkUpdate:
		if operation.id == "" {
			err = errors.New("id is required for update operation")
		} else {
			err = r.UpdateOp(types.UpdateOperation{Index_: &operation.index, Id_: &operation.id}, operation.document, types.NewUpdateAction())
		}
	}

	if err != nil {
		return fmt.Errorf("could not process bulk operation: %w", err)
	}

	return nil
}

// BulkOperation represents an individual document's bulk operation.
type BulkOperation struct {
	document any
	index    string
	opType   BulkOpType
	id       string
}

func SetDocID(id string) BulkOpOption {
	return func(operation *BulkOperation) {
		operation.id = id
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

// NewOperation creates a new bulk operation for a document with the given options.
func NewOperation(doc any, options ...BulkOpOption) BulkOperation {
	operation := &BulkOperation{
		document: doc,
	}

	for _, option := range options {
		option(operation)
	}

	return *operation
}

// NewRequest creates a new bulk requesst object with the given options.
// After creation, document operations can be added with the AddOperations method.
func NewRequest(ctx context.Context, client Client, options ...BulkOption) (chan BulkOperation, chan error) {
	req := &BulkRequest{
		Bulk: client.GetAPI().Bulk(),
	}

	for _, option := range options {
		option(req)
	}

	bulkOps := make(chan BulkOperation)
	errorCh := make(chan error)

	go func() {
		defer close(errorCh)

		for op := range bulkOps {
			if err := req.AddOperation(op); err != nil {
				client.Log().Warn("Could not add operation to bulk request.",
					slog.Any("error", err))
			}
		}

		resp, err := req.Do(ctx)
		// Handle response.
		switch {
		case err != nil:
			errorCh <- fmt.Errorf("bulk index failed: %w", err)
		case resp.Errors:
			errorCh <- fmt.Errorf("bulk index completed with some errors: %w", resp.Items)
		default:
			errorCh <- nil
		}
	}()

	return bulkOps, errorCh
}
