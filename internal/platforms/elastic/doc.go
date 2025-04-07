// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"encoding/json"
	"log/slog"

	"github.com/elastic/go-elasticsearch/v8/typedapi"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/create"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/delete"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/exists"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/update"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/refresh"
)

// DocUpdateOption is a functional option for a DocUpdateRequest.
type DocUpdateOption Option[*DocUpdateRequest]

type HasForcedRefreshOption interface {
	ForceRefresh()
}

type DocUpdateRequest struct {
	*update.Request
	forceRefresh bool
}

func (u *DocUpdateRequest) ForceRefresh() {
	u.forceRefresh = true
}

// DetectNoop configures noop handling. Set to false to disable setting 'result'
// in the response to 'noop' if no change to the document occurred.
func DetectNoop(value bool) DocUpdateOption {
	return func(update *DocUpdateRequest) {
		update.DetectNoop = &value
	}
}

// WithDocUpdate sets the full document content for the update. The document is
// automatically marshaled to JSON.
func WithDocUpdate(doc any, upsert bool) DocUpdateOption {
	return func(update *DocUpdateRequest) {
		data, err := json.Marshal(doc)
		if err != nil {
			slog.Warn("Could not marshal document.", slog.Any("error", err))
			return
		}

		update.Doc = data

		if upsert {
			update.DocAsUpsert = &upsert
		}
	}
}

// WithPartialDocUpdate sets the fields of the document to update. The fields are
// automatically marshaled to JSON.
func WithPartialDocUpdate(fields map[string]any) DocUpdateOption {
	return func(update *DocUpdateRequest) {
		data, err := json.Marshal(fields)
		if err != nil {
			slog.Warn("Could not marshal fields.", slog.Any("error", err))
			return
		}

		update.Doc = data
	}
}

func WithForcedRefresh() DocUpdateOption {
	return func(req *DocUpdateRequest) {
		req.ForceRefresh()
	}
}

// NewDocUpdateRequest creates a new update request with the given options.
func NewDocUpdateRequest(api *typedapi.API, index, id string, options ...DocUpdateOption) *update.Update {
	req := &DocUpdateRequest{
		Request: update.NewRequest(),
	}

	for _, option := range options {
		option(req)
	}

	return api.Update(index, id).Request(req.Request)
}

// NewDocExistsRequest creates a new document exists request.
func NewDocExistsRequest(api *typedapi.API, index, id string) *exists.Exists {
	return api.Exists(index, id)
}

// NewExistsRequest creates a new document index request.
func NewDocCreateRequest(api *typedapi.API, index, id string, doc any, refreshValue refresh.Refresh) *create.Create {
	return api.Create(index, id).Document(doc).Refresh(refreshValue)
}

// NewExistsRequest creates a new document delete request.
func NewDocDeleteRequest(api *typedapi.API, index, id string, refreshValue refresh.Refresh) *delete.Delete {
	return api.Delete(index, id).Refresh(refreshValue)
}
