// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package bulk contains methods and structures for handling Elasticsearch bulk operations.
package bulk

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/elastic/go-elasticsearch/v8/typedapi"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/bulk"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/operationtype"
	"github.com/go-chi/chi/v5/middleware"
	slogctx "github.com/veqryn/slog-context"
)

var (
	// ErrCreateOpFailed indicates creating a bulk operation failed.
	ErrCreateOpFailed = errors.New("could not create bulk operation")
	// ErrOpFailed indicates executing a bulk operation failed.
	ErrOpFailed = errors.New("bulk operation failed")
	// ErrBulkHasErrors indicates the bulk request completed but some operations produced an error.
	ErrBulkHasErrors = errors.New("bulk request completed with some operation errors")
)

const (
	// BulkCreate is a create operation.
	BulkCreate OpType = iota
	// BulkIndex is a index operation.
	BulkIndex
	// BulkDelete is a delete operation.
	BulkDelete
	// BulkUpdate is an update operation.
	BulkUpdate
)

// Client represents an elasticsearch client connection.
type Client interface {
	GetAPI() *typedapi.API
}

// OpType represents the type of bulk operation to perform.
type OpType int

// OperationOption is a functional option for a bulk operation.
type OperationOption func(*Operation)

// Option is a functional option for a bulk request.
type Option func(*Request)

// Request represents a bulk request.
type Request struct {
	*bulk.Bulk
}

// WithPipeline defines the ingest pipeline to use on each document.
func WithPipeline(pipeline string) Option {
	return func(b *Request) {
		if pipeline != "" {
			b.Pipeline(pipeline)
		}
	}
}

// WithIndex defines the index on which the request will operate.
func WithIndex(index string) Option {
	return func(b *Request) {
		b.Index(index)
	}
}

// AddOperation adds document operations to the bulk request.
func (r *Request) AddOperation(operation Operation) error {
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
			return fmt.Errorf("%w: a doc id is required", ErrCreateOpFailed)
		}
		err = r.UpdateOp(types.UpdateOperation{Index_: &operation.index, Id_: &operation.id}, operation.document, types.NewUpdateAction())
	}

	if err != nil {
		return fmt.Errorf("%w: %w", ErrCreateOpFailed, err)
	}

	return nil
}

// Response holds details about a bulk request's results.
type Response struct {
	Err       error
	Responses []*OperationResponse
}

// FailedDocs returns the doc IDs of all documents whose operation failed.
func (r *Response) FailedDocs() []string {
	var failedDocIDs []string
	for _, resp := range r.Responses {
		// Ignore unknown operation responses.
		if resp.Id_ == nil {
			continue
		}
		if _, err := resp.State(); err != nil {
			failedDocIDs = append(failedDocIDs, *resp.Id_)
		}
	}
	return failedDocIDs
}

// OperationResponse holds details about a bulk operation's result.
type OperationResponse struct {
	*types.ResponseItem
}

func (r *OperationResponse) Created() bool {
	return r.Result != nil && *r.Result == "created"
}

func (r *OperationResponse) Updated() bool {
	return r.Result != nil && *r.Result == "updated"
}

func (r *OperationResponse) Deleted() bool {
	return r.Result != nil && *r.Result == "deleted"
}

// State returns a string indicating the operation result and a non-nil error if one occurred.
func (r *OperationResponse) State() (string, error) {
	var status string
	if r.Result != nil {
		status = fmt.Sprintf("%d: %s", r.Status, *r.Result)
	} else {
		status = strconv.Itoa(r.Status)
	}

	var err error
	if r.Error != nil {
		if r.Error.Reason != nil {
			err = fmt.Errorf("%w: %s (%s)", ErrOpFailed, r.Error.Type, *r.Error.Reason)
		} else {
			err = fmt.Errorf("%w: %s", ErrOpFailed, r.Error.Type)
		}
	}

	return status, err
}

// Operation represents an individual document's bulk operation.
type Operation struct {
	document any
	index    string
	opType   OpType
	id       string
}

// SetDocID option sets the doc id for the operation.
func SetDocID(id string) OperationOption {
	return func(operation *Operation) {
		operation.id = id
	}
}

// AsOperationType option specifies the type of bulk operation to perform. If this option is
// not specified, the operation will default to a create operation.
func AsOperationType(opType OpType) OperationOption {
	return func(operation *Operation) {
		operation.opType = opType
	}
}

// ToIndex option sets the index containing the document.
func ToIndex(index string) OperationOption {
	return func(operation *Operation) {
		operation.index = index
	}
}

// NewOperation creates a new bulk operation for a document with the given options.
func NewOperation(doc any, options ...OperationOption) Operation {
	operation := &Operation{
		document: doc,
	}

	for _, option := range options {
		option(operation)
	}

	return *operation
}

// NewRequest creates a new bulk requesst object with the given options.
// After creation, document operations can be added with the AddOperations method.
func NewRequest(ctx context.Context, client Client, options ...Option) (chan Operation, chan Response) {
	req := &Request{
		Bulk: client.GetAPI().Bulk(),
	}

	for _, option := range options {
		option(req)
	}

	bulkOps := make(chan Operation)
	respCh := make(chan Response)

	go func() {
		defer close(respCh)

		for op := range bulkOps {
			if err := req.AddOperation(op); err != nil {
				logger(ctx).Warn("Could not add operation to bulk request.",
					slog.Any("error", err))
			}
		}

		resp, err := req.Do(ctx)
		// Handle response.
		switch {
		case err != nil:
			logger(ctx).Error("Bulk request failed.", slog.Any("error", err))
			respCh <- Response{Err: err}
		case resp.Errors:
			logger(ctx).Warn("Bulk request completed with some operation errors.")
			respCh <- Response{Err: ErrBulkHasErrors, Responses: GetOperationResponses(resp.Items)}
		default:
			respCh <- Response{Responses: GetOperationResponses(resp.Items)}
		}
	}()

	return bulkOps, respCh
}

// GetOperationResponses extracts a slice of OperationResponse from the bulk request response data.
func GetOperationResponses(resp []map[operationtype.OperationType]types.ResponseItem) []*OperationResponse {
	responses := make([]*OperationResponse, 0, len(resp))
	for _, op := range resp {
		for _, doc := range op {
			responses = append(responses, &OperationResponse{ResponseItem: &doc})
		}
	}
	return responses
}

func logger(ctx context.Context) *slog.Logger {
	logger := slogctx.FromCtx(ctx)
	if id := middleware.GetReqID(ctx); id != "" {
		logger = logger.With(slog.String("id", id))
	}
	return logger.WithGroup("bulk")
}
