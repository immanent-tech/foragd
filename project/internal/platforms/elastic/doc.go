// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/create"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/exists"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/update"
)

var (
	ErrUpdateFailed = errors.New("update failed")
	ErrExistsFailed = errors.New("exists request failed")
)

// DetectNoop configures noop handling. Set to false to disable setting 'result'
// in the response to 'noop' if no change to the document occurred.
func DetectNoop(value bool) Option[*update.Request] {
	return func(update *update.Request) *update.Request {
		update.DetectNoop = &value
		return update
	}
}

// WithDocUpdate sets the document content for the update. This can be a partial
// or full document. The document is automatically marshaled to JSON.
func WithDocUpdate(doc any) Option[*update.Request] {
	return func(update *update.Request) *update.Request {
		data, err := json.Marshal(doc)
		if err != nil {
			slog.Warn("Could not marshal document.", slog.Any("error", err))
			return update
		}

		update.Doc = data

		return update
	}
}

// NewUpdateRequest creates a new update request with the given options.
func (c *Client) NewUpdateRequest(index, id string, options ...Option[*update.Request]) *update.Update {
	req := &update.Request{}

	for _, option := range options {
		req = option(req)
	}

	return c.API.Update(index, id).Request(req)
}

// NewExistsRequest creates a new exists request.
func (c *Client) NewExistsRequest(index, id string) *exists.Exists {
	return c.API.Exists(index, id)
}

// NewExistsRequest creates a new exists request.
func (c *Client) NewCreateRequest(index, id string, doc any) *create.Create {
	return c.API.Create(index, id).Document(doc)
}
