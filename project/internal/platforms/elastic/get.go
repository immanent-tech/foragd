// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"errors"

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/get"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/mget"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

var ErrGetFailed = errors.New("get request failed")

// WithDocID specifies the document ID to get.
func WithGetDocID(id string) Option[types.MgetOperation] {
	return func(mo types.MgetOperation) types.MgetOperation {
		mo.Id_ = id
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
