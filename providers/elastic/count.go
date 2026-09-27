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
		WithIndex[*count.Count](index),
		WithQuery[*count.Count](queries...),
	).Do(ctx)
	if err != nil {
		return 0, fmt.Errorf("count: %w", err)
	}

	return resp.Count, nil
}

// NewCountRequest creates a new count request with the given options.
func NewCountRequest(
	ctx context.Context,
	api *elasticsearch.TypedClient,
	options ...Option[*count.Count],
) *count.Count {
	req := api.Count()
	req = WithHeader[*count.Count](ReqIDHeader, middleware.GetReqID(ctx))(req)
	for _, option := range options {
		option(req)
	}
	return req
}
