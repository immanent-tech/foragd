// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/get"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/mget"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

// WithDocID specifies the document ID to get.
func WithDocID(id string) Option[types.MgetOperation] {
	return func(mo types.MgetOperation) types.MgetOperation {
		mo.Id_ = id
		return mo
	}
}

// FromIndex specifies the index containing the document.
func FromIndex(index string) Option[types.MgetOperation] {
	return func(mo types.MgetOperation) types.MgetOperation {
		mo.Index_ = &index
		return mo
	}
}

// GetDoc creates a get doc operation.
func GetDoc(options ...Option[types.MgetOperation]) types.MgetOperation {
	req := types.MgetOperation{}

	for _, option := range options {
		req = option(req)
	}

	return req
}

// WithDocs adds get operations for all the specified doc IDs to the request.
func WithDocs(ids ...string) Option[*mget.Mget] {
	return func(mget *mget.Mget) *mget.Mget {
		// Create get ops for all the specified IDs.
		var docs []types.MgetOperation
		for _, id := range ids {
			docs = append(docs, GetDoc(
				WithDocID(id),
			))
		}
		// Add the ops to the request.
		mget = mget.Docs(docs...)

		return mget
	}
}

// WithIDs retrieves the documents with the given IDs.
func WithIDs(ids ...string) Option[*mget.Mget] {
	return func(m *mget.Mget) *mget.Mget {
		m = m.Ids(ids...)
		return m
	}
}

func WithStoredFields(fields ...string) Option[*mget.Mget] {
	return func(m *mget.Mget) *mget.Mget {
		m = m.StoredFields(fields...)
		return m
	}
}

// NewMGetRequest creates a new mget object with the given options.
func (c *Client) NewMGetRequest(options ...Option[*mget.Mget]) *mget.Mget {
	req := c.API.Mget()

	for _, option := range options {
		req = option(req)
	}

	return req
}

func (c *Client) NewGetRequest(index, id string) *get.Get {
	return c.API.Get(index, id)
}
