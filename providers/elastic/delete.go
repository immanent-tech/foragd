// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"slices"

	elasticsearch "github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/deletebyquery"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/immanent-tech/foragd/providers/elastic/query"
)

type DeleteByQueryRequest struct {
	*deletebyquery.DeleteByQuery
}

// NewDeleteByQueryRequest creates a new delete by query request that will operate on the given index with the given
// options.
func NewDeleteByQueryRequest(
	ctx context.Context,
	api *elasticsearch.TypedClient,
	index string,
	options ...func(*DeleteByQueryRequest),
) *DeleteByQueryRequest {
	req := &DeleteByQueryRequest{
		DeleteByQuery: api.DeleteByQuery(index),
	}

	WithHeader[*DeleteByQueryRequest](ReqIDHeader, middleware.GetReqID(ctx))(req)

	for option := range slices.Values(options) {
		option(req)
	}

	return req
}

func (r *DeleteByQueryRequest) SetHeader(key, value string) {
	r.DeleteByQuery = r.Header(key, value)
}

func (r *DeleteByQueryRequest) SetQueryOptions(options ...query.Option) {
	if query := query.Build(options...); query != nil {
		r.DeleteByQuery = r.Query(query)
	}
}
