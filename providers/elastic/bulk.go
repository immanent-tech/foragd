// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"

	"github.com/immanent-tech/foragd/providers/elastic/bulk"
)

var ErrNotFound = errors.New("not found")

var (
	_ types.FieldValueVariant = (*paginationValue[types.FieldValue])(nil)
)

// Object represents any kind of object that has an ID. Effectively the object can be indexed in Elasticsearch.
type Object[T ~string] interface {
	GetID() T
}

// Option is a generic type for functional options.
type Option[T any] func(T)

// BulkAdd will create documents for the given list of objects. Responses are returned as a map of doc id to response.
// If the request itself fails, a non-nil error is returned.
func BulkAdd[T ~string, O Object[T]](
	ctx context.Context,
	index string,
	objects ...O,
) (map[T]*bulk.OperationResponse, error) {
	// Connect to elasticsearch (if not already connected).
	if err := Connect(); err != nil {
		return nil, fmt.Errorf("connect to elasticsearch: %w", err)
	}

	bulkOps, respCh := bulk.NewRequest(ctx, api.TypedClient)

	// Always require operating against an index alias for non logs bulk requests.
	var requireAlias bool
	if index != "logs" {
		requireAlias = true
	}

	go func() {
		defer close(bulkOps)

		for object := range slices.Values(objects) {
			bulkOps <- bulk.NewOperation(object,
				bulk.SetDocID(string(object.GetID())),
				bulk.ToIndex(index),
				bulk.RequireIndexAlias(requireAlias),
			)
		}
	}()

	bulkOpResponse := <-respCh
	// If the request failed, return an error.
	if bulkOpResponse.Err != nil {
		return nil, fmt.Errorf("bulk add: %w", bulkOpResponse.Err)
	}
	// Create a map of responses by object id.
	responses := make(map[T]*bulk.OperationResponse)
	// Map responses to object id.
	for opResp := range slices.Values(bulkOpResponse.Responses) {
		if opResp.Id_ == nil {
			continue
		}
		if idx := slices.IndexFunc(objects, func(o O) bool {
			return string(o.GetID()) == *opResp.Id_
		}); idx != -1 {
			responses[objects[idx].GetID()] = opResp
		}
	}

	return responses, nil
}

// BulkUpdate will update documents for the given list of objects. Responses are returned as a map of doc id to response.
// If the request itself fails, a non-nil error is returned.
func BulkUpdate[T ~string, O Object[T]](
	ctx context.Context,
	index string,
	objects ...O,
) (map[T]*bulk.OperationResponse, error) {
	// Connect to elasticsearch (if not already connected).
	if err := Connect(); err != nil {
		return nil, fmt.Errorf("connect to elasticsearch: %w", err)
	}

	bulkOps, respCh := bulk.NewRequest(ctx, api.TypedClient)

	go func() {
		defer close(bulkOps)

		for object := range slices.Values(objects) {
			bulkOps <- bulk.NewOperation(object,
				bulk.AsOperationType(bulk.BulkUpdate),
				bulk.SetDocID(string(object.GetID())),
				bulk.ToIndex(index),
				bulk.Upsert(true),
			)
		}
	}()

	bulkOpResponse := <-respCh
	// If the request failed, return an error.
	if bulkOpResponse.Err != nil {
		return nil, fmt.Errorf("bulk update: %w", bulkOpResponse.Err)
	}
	// Create  a map of responses by object id.
	responses := make(map[T]*bulk.OperationResponse)
	// Map responses to object id.
	for opResp := range slices.Values(bulkOpResponse.Responses) {
		if opResp.Id_ == nil {
			continue
		}
		if idx := slices.IndexFunc(objects, func(o O) bool {
			return string(o.GetID()) == *opResp.Id_
		}); idx != -1 {
			responses[objects[idx].GetID()] = opResp
		}
	}

	return responses, nil
}
