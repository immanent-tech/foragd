// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	elasticsearch "github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/create"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/delete"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/get"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/mget"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/update"
	"github.com/go-chi/chi/v5/middleware"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/logging"

	"github.com/immanent-tech/foragd/providers/elastic/results"
)

// GetDocs performs an `_mget` request to fetch the documents from the given index with the given ids. A non-nil error
// is returned on a failure.
func GetDocs[T ~string, O any](
	ctx context.Context,
	index string,
	ids ...T,
) ([]O, error) {
	// Connect to elasticsearch (if not already connected).
	if err := Connect(); err != nil {
		return nil, fmt.Errorf("connect to elasticsearch: %w", err)
	}

	docIDs := make([]string, 0, len(ids))
	for id := range slices.Values(ids) {
		docIDs = append(docIDs, string(id))
	}
	resp, err := NewMGetRequest(ctx, api.TypedClient,
		WithIndex[*mget.Mget](index),
		WithDocIDs[*mget.Mget](docIDs...),
	).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("get docs: %w", err)
	}
	objects, warnings := results.ExtractSourceFromDocs[O](resp.Docs)
	if warnings != nil {
		slogctx.FromCtx(ctx).WarnContext(ctx, "Some docs could not be extracted.",
			slog.Any("warnings", warnings))
	}
	return objects, nil
}

// GetDoc retrieves the doc with the given id from the given index. A non-nil error is returned on a failure.
func GetDoc[T ~string, O any](ctx context.Context, index string, id T) (O, error) {
	var doc O

	// Connect to elasticsearch (if not already connected).
	if err := Connect(); err != nil {
		return doc, fmt.Errorf("connect to elasticsearch: %w", err)
	}

	resp, err := NewGetRequest(ctx, api.TypedClient, index, string(id)).Do(ctx)
	if err != nil {
		return doc, fmt.Errorf("get doc: %w", err)
	}
	if !resp.Found {
		return doc, ErrNotFound
	}
	doc, err = results.ExtractSource[O](resp.Source_)
	if err != nil {
		return doc, fmt.Errorf("extract doc source: %w", err)
	}
	return doc, nil
}

// CreateDoc will create the given document, with given id, in the given index.
func CreateDoc[T ~string, O any](
	ctx context.Context,
	index string,
	id T,
	doc O,
	options ...Option[*create.Create],
) error {
	// Connect to elasticsearch (if not already connected).
	if err := Connect(); err != nil {
		return AsAPIError(fmt.Errorf("connect to elasticsearch: %w", err))
	}

	req := api.Create(index, string(id)).
		Document(doc).
		Header(ReqIDHeader, middleware.GetReqID(ctx))

	for option := range slices.Values(options) {
		option(req)
	}

	resp, err := req.Do(ctx)
	if err != nil {
		return AsAPIError(err)
	}
	if resp != nil {
		slogctx.FromCtx(ctx).Log(ctx, logging.LevelTrace, "Created document.",
			slog.String("id", resp.Id_),
			slog.String("result", resp.Result.String()),
		)
	}
	return nil
}

// UpdateDoc performs a partial doc update on the document with the given id in the given index. A non-nil error is
// returned on a failure.
func UpdateDoc[T ~string](
	ctx context.Context,
	index string,
	id T,
	updates any,
	options ...Option[*update.Update],
) error {
	// Connect to elasticsearch (if not already connected).
	if err := Connect(); err != nil {
		return fmt.Errorf("connect to elasticsearch: %w", err)
	}

	resp, err := NewUpdateDocRequest(ctx, api.TypedClient, index, string(id), updates, options...).Do(ctx)
	if err != nil {
		return fmt.Errorf("update doc: %w", err)
	}
	if resp != nil {
		slogctx.FromCtx(ctx).Log(ctx, logging.LevelTrace, "Updated document.",
			slog.String("doc_id", resp.Id_),
			slog.String("result", resp.Result.String()),
		)
	}

	return nil
}

// DeleteDoc deletes the document with the given id from the given index.
func DeleteDoc[T ~string](ctx context.Context, index string, id T, options ...Option[*delete.Delete]) error {
	if err := Connect(); err != nil {
		return AsAPIError(fmt.Errorf("connect to elasticsearch: %w", err))
	}

	req := api.Delete(index, string(id)).Header(ReqIDHeader, middleware.GetReqID(ctx))
	for option := range slices.Values(options) {
		req = option(req)
	}

	resp, err := req.Do(ctx)
	if err != nil {
		return AsAPIError(err)
	}
	if resp != nil {
		slogctx.FromCtx(ctx).Log(ctx, logging.LevelTrace, "Deleted document.",
			slog.String("id", resp.Id_),
			slog.String("result", resp.Result.String()),
		)
	}
	return nil
}

// NewGetRequest creates a new get object with the given options.
func NewGetRequest(
	ctx context.Context,
	api *elasticsearch.TypedClient,
	index string,
	id string,
	options ...Option[*get.Get],
) *get.Get {
	req := api.Get(index, id)
	req = WithHeader[*get.Get](ReqIDHeader, middleware.GetReqID(ctx))(req)
	for option := range slices.Values(options) {
		option(req)
	}
	return req
}

// NewMGetRequest creates a new mget object with the given options.
func NewMGetRequest(ctx context.Context, api *elasticsearch.TypedClient, options ...Option[*mget.Mget]) *mget.Mget {
	req := api.Mget()

	req = WithHeader[*mget.Mget](ReqIDHeader, middleware.GetReqID(ctx))(req)
	for option := range slices.Values(options) {
		option(req)
	}

	return req
}

// NewUpdateDocRequest creates a new doc update request with the given options.
func NewUpdateDocRequest(
	ctx context.Context,
	api *elasticsearch.TypedClient,
	index, id string,
	doc any,
	options ...Option[*update.Update],
) *update.Update {
	req := api.Update(index, id).Doc(doc)

	req = WithHeader[*update.Update](ReqIDHeader, middleware.GetReqID(ctx))(req)
	for _, option := range options {
		option(req)
	}

	return req
}
