// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	elasticsearch "github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/deletebyquery"
	"github.com/go-chi/chi/v5/middleware"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/logging"
	"github.com/immanent-tech/foragd/providers/elastic/query"
)

// DeleteDocs performs a delete by query request on the given index to delete documents matching the given queries.
func DeleteDocs(ctx context.Context, index string, queries ...query.Option) error {
	// Connect to elasticsearch (if not already connected).
	if err := Connect(); err != nil {
		return fmt.Errorf("connect to elasticsearch: %w", err)
	}

	resp, err := NewDeleteByQueryRequest(ctx,
		api.TypedClient,
		index,
		WithQueryOptions[*DeleteByQueryRequest](queries...),
	).Do(ctx)
	if err != nil {
		return fmt.Errorf("delete docs: %w", err)
	}
	if resp != nil {
		slogctx.FromCtx(ctx).Log(ctx, logging.LevelTrace, "Delete documents.",
			slog.Int64("count", *resp.Deleted),
		)
	}
	return nil
}

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
