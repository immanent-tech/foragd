// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/create"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/delete"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/exists"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/update"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/refresh"
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

// WithDocUpdate sets the full document content for the update. The document is
// automatically marshaled to JSON.
func WithDocUpdate(doc any, upsert bool) Option[*update.Request] {
	return func(update *update.Request) *update.Request {
		data, err := json.Marshal(doc)
		if err != nil {
			slog.Warn("Could not marshal document.", slog.Any("error", err))
			return update
		}

		update.Doc = data

		if upsert {
			update.DocAsUpsert = &upsert
		}

		return update
	}
}

// WithPartialDocUpdate sets the fields of the document to update. The fields are
// automatically marshaled to JSON.
func WithPartialDocUpdate(fields map[string]any) Option[*update.Request] {
	return func(update *update.Request) *update.Request {
		data, err := json.Marshal(fields)
		if err != nil {
			slog.Warn("Could not marshal fields.", slog.Any("error", err))
			return update
		}

		update.Doc = data

		return update
	}
}

// NewDocUpdateRequest creates a new update request with the given options.
func (c *Client) NewDocUpdateRequest(index, id string, options ...Option[*update.Request]) *update.Update {
	req := &update.Request{}

	for _, option := range options {
		req = option(req)
	}

	return c.API.Update(index, id).Request(req)
}

// NewDocExistsRequest creates a new document exists request.
func (c *Client) NewDocExistsRequest(index, id string) *exists.Exists {
	return c.API.Exists(index, id)
}

// NewExistsRequest creates a new document index request.
func (c *Client) NewDocCreateRequest(index, id string, doc any, refreshValue refresh.Refresh) *create.Create {
	return c.API.Create(index, id).Document(doc).Refresh(refreshValue)
}

// NewExistsRequest creates a new document delete request.
func (c *Client) NewDocDeleteRequest(index, id string, refreshValue refresh.Refresh) *delete.Delete {
	return c.API.Delete(index, id).Refresh(refreshValue)
}
