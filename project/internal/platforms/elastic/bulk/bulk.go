// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

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
type BulkOpOption func(*Operation)

// BulkOption is a functional option for a bulk request.
type BulkOption func(*Request)

// Request represents a bulk request.
type Request struct {
	*bulk.Bulk
}

// WithPipeline defines the ingest pipeline to use on each document.
func WithPipeline(pipeline string) BulkOption {
	return func(b *Request) {
		if pipeline != "" {
			b.Pipeline(pipeline)
		}
	}
}

// WithIndex defines the index on which the request will operate.
func WithOverallIndex(index string) BulkOption {
	return func(b *Request) {
		b.Index(index)
	}
}

// AddOperations adds document operations to the bulk request.
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

type OperationResponse struct {
	*types.ResponseItem
}

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
			err = fmt.Errorf("%s: %s", r.Error.Type, *r.Error.Reason)
		} else {
			err = fmt.Errorf("%s", r.Error.Type)
		}
	}

	return status, err
}

// Operation represents an individual document's bulk operation.
type Operation struct {
	document any
	index    string
	opType   BulkOpType
	id       string
}

func SetDocID(id string) BulkOpOption {
	return func(operation *Operation) {
		operation.id = id
	}
}

// AsOperationType specifies the type of bulk operation to perform. If this option is
// not specified, the operation will default to a create operation.
func AsOperationType(opType BulkOpType) BulkOpOption {
	return func(operation *Operation) {
		operation.opType = opType
	}
}

// ToIndex sets the index containing the document.
func ToIndex(index string) BulkOpOption {
	return func(operation *Operation) {
		operation.index = index
	}
}

// NewOperation creates a new bulk operation for a document with the given options.
func NewOperation(doc any, options ...BulkOpOption) Operation {
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
func NewRequest(ctx context.Context, client Client, options ...BulkOption) (chan Operation, chan Response) {
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
				client.Log().Warn("Could not add operation to bulk request.",
					slog.Any("error", err))
			}
		}

		resp, err := req.Do(ctx)
		// Handle response.
		switch {
		case err != nil:
			respCh <- Response{Err: err}
		case resp.Errors:
			respCh <- Response{Err: errors.New("bulk request completed with some errors"), Responses: GetOperationResponses(resp.Items)}
		default:
			respCh <- Response{Responses: GetOperationResponses(resp.Items)}
		}
	}()

	return bulkOps, respCh
}

func GetOperationResponses(resp []map[operationtype.OperationType]types.ResponseItem) []*OperationResponse {
	responses := make([]*OperationResponse, 0, len(resp))
	for _, op := range resp {
		for _, doc := range op {
			responses = append(responses, &OperationResponse{ResponseItem: &doc})
		}
	}
	return responses
}
