// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package bulk

import (
	"errors"
	"fmt"

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

type Option[T any] func(T)

// BulkOpType represents the type of bulk operation to perform.
type BulkOpType int

// BulkOpOption is a functional option for a bulk operation.
type BulkOpOption Option[*BulkOperation]

// BulkOption is a functional option for a bulk request.
type BulkOption Option[*BulkRequest]

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

// NewOp creates a new bulk operation for a document with the given options.
func NewOp(doc any, options ...BulkOpOption) BulkOperation {
	operation := &BulkOperation{
		document: doc,
	}

	for _, option := range options {
		option(operation)
	}

	return *operation
}
