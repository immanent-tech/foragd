// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"fmt"

	elasticsearch "github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/count"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/immanent-tech/foragd/providers/elastic/query"
)

// Count will return the number of docs matching the given queries in the given index.
func Count(ctx context.Context, index string, queries ...query.Option) (int64, error) {
	// Connect to elasticsearch (if not already connected).
	if err := Connect(); err != nil {
		return 0, fmt.Errorf("connect to elasticsearch: %w", err)
	}

	resp, err := NewCountRequest(ctx, api.TypedClient,
		WithIndex[*CountRequest](index),
		WithQueryOptions[*CountRequest](queries...),
	).Do(ctx)
	if err != nil {
		return 0, fmt.Errorf("count: %w", err)
	}

	return resp.Count, nil
}

type CountRequest struct {
	*count.Count
}

// NewCountRequest creates a new count request with the given options.
func NewCountRequest(
	ctx context.Context,
	api *elasticsearch.TypedClient,
	options ...func(*CountRequest),
) *CountRequest {
	req := &CountRequest{
		Count: api.Count(),
	}

	WithHeader[*CountRequest](ReqIDHeader, middleware.GetReqID(ctx))(req)

	for _, option := range options {
		option(req)
	}

	return req
}

func (r *CountRequest) SetHeader(key, value string) {
	r.Count = r.Header(key, value)
}

func (r *CountRequest) SetIndex(index string) {
	r.Count = r.Index(index)
}

func (r *CountRequest) SetQueryOptions(options ...query.Option) {
	if query := query.Build(options...); query != nil {
		r.Count = r.Query(query)
	}
}
