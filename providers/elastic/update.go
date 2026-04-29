// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"

	elasticsearch "github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/update"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/refresh"
	"github.com/go-chi/chi/v5/middleware"
)

type UpdateRequest struct {
	*update.Update
}

// NewUpdateDocRequest creates a new doc update request with the given options.
func NewUpdateDocRequest(
	ctx context.Context,
	api *elasticsearch.TypedClient,
	index, id string,
	doc any,
	options ...func(*UpdateRequest),
) *UpdateRequest {
	req := &UpdateRequest{
		Update: api.Update(index, id).Doc(doc),
	}

	WithHeader[*UpdateRequest](ReqIDHeader, middleware.GetReqID(ctx))(req)

	for _, option := range options {
		option(req)
	}

	return req
}

func (r *UpdateRequest) SetHeader(key, value string) {
	r.Update = r.Header(key, value)
}

func (r *UpdateRequest) SetRefresh(value refresh.Refresh) {
	r.Update = r.Refresh(value)
}

func (r *UpdateRequest) SetDocAsUpsert(value bool) {
	r.Update = r.DocAsUpsert(value)
}

func (r *UpdateRequest) SetRetryOnConflict(retries int) {
	r.Update = r.RetryOnConflict(retries)
}
