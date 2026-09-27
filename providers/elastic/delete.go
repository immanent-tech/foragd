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

	"github.com/immanent-tech/go-base/logging"

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
		WithQuery[*deletebyquery.DeleteByQuery](queries...),
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

// NewDeleteByQueryRequest creates a new delete by query request that will operate on the given index with the given
// options.
func NewDeleteByQueryRequest(
	ctx context.Context,
	api *elasticsearch.TypedClient,
	index string,
	options ...Option[*deletebyquery.DeleteByQuery],
) *deletebyquery.DeleteByQuery {
	req := api.DeleteByQuery(index)
	req = WithHeader[*deletebyquery.DeleteByQuery](ReqIDHeader, middleware.GetReqID(ctx))(req)
	for option := range slices.Values(options) {
		option(req)
	}
	return req
}
