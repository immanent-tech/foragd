// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"github.com/elastic/go-elasticsearch/v8/typedapi"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/get"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/mget"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

// MGetOperationOption is a functional option for an mget operation.
type MGetOperationOption Option[*types.MgetOperation]

// MGetOption is a functional option for an mget request.
type MGetOption Option[*mget.Mget]

// WithDocID specifies the document ID to get.
func WithGetDocID(id string) MGetOperationOption {
	return func(mo *types.MgetOperation) {
		mo.Id_ = id
	}
}

// GetDoc creates a get doc operation.
func GetDoc(options ...MGetOperationOption) *types.MgetOperation {
	req := &types.MgetOperation{}

	for _, option := range options {
		option(req)
	}

	return req
}

// GetIDs option sets the document IDs to get.
func GetIDs(ids ...string) MGetOption {
	return func(m *mget.Mget) {
		m.Ids(ids...)
	}
}

// GetFromIndex option sets the index (or index pattern) to get documents from.
func GetFromIndex(index string) MGetOption {
	return func(m *mget.Mget) {
		m.Index(index)
	}
}

// NewMGetRequest creates a new mget object with the given options.
func NewMGetRequest(api *typedapi.API, options ...MGetOption) *mget.Mget {
	req := api.Mget()

	for _, option := range options {
		option(req)
	}

	return req
}

func NewGetRequest(api *typedapi.API, index, id string) *get.Get {
	return api.Get(index, id)
}
