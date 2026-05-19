// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	elasticsearch "github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/get"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/mget"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/update"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/refresh"
	"github.com/go-chi/chi/v5/middleware"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/logging"
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
		WithIndex[*MGetRequest](index),
		WithIDs(docIDs...),
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
		return doc, fmt.Errorf("get doc: extract doc: %w", err)
	}
	return doc, nil
}

// CreateDoc will create the given document, with given id, in the given index.
func CreateDoc[T ~string, O any](ctx context.Context, index string, id T, doc O) error {
	// Connect to elasticsearch (if not already connected).
	if err := Connect(); err != nil {
		return fmt.Errorf("connect to elasticsearch: %w", err)
	}

	resp, err := api.Create(index, string(id)).
		Document(doc).
		Header(ReqIDHeader, middleware.GetReqID(ctx)).
		Refresh(refresh.True).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("create doc: %w", err)
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
	options ...func(*UpdateRequest),
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
func DeleteDoc[T ~string](ctx context.Context, index string, id T) error {
	// Connect to elasticsearch (if not already connected).
	if err := Connect(); err != nil {
		return fmt.Errorf("connect to elasticsearch: %w", err)
	}

	resp, err := api.Delete(index, string(id)).
		Header(ReqIDHeader, middleware.GetReqID(ctx)).
		Refresh(refresh.True).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("delete doc: %w", err)
	}
	if resp != nil {
		slogctx.FromCtx(ctx).Log(ctx, logging.LevelTrace, "Deleted document.",
			slog.String("id", resp.Id_),
			slog.String("result", resp.Result.String()),
		)
	}
	return nil
}

type GetRequest struct {
	*get.Get
}

// NewGetRequest creates a new get object with the given options.
func NewGetRequest(
	ctx context.Context,
	api *elasticsearch.TypedClient,
	index string,
	id string,
	options ...func(*GetRequest),
) *GetRequest {
	req := &GetRequest{
		Get: api.Get(index, id),
	}

	WithHeader[*GetRequest](ReqIDHeader, middleware.GetReqID(ctx))(req)

	for option := range slices.Values(options) {
		option(req)
	}

	return req
}

func (r *GetRequest) SetHeader(key, value string) {
	r.Get = r.Header(key, value)
}

type MGetRequest struct {
	*mget.Mget
}

// NewMGetRequest creates a new mget object with the given options.
func NewMGetRequest(ctx context.Context, api *elasticsearch.TypedClient, options ...func(*MGetRequest)) *MGetRequest {
	req := &MGetRequest{
		Mget: api.Mget(),
	}

	WithHeader[*MGetRequest](ReqIDHeader, middleware.GetReqID(ctx))(req)

	for option := range slices.Values(options) {
		option(req)
	}

	return req
}

func (r *MGetRequest) SetHeader(key, value string) {
	r.Mget = r.Header(key, value)
}

func (r *MGetRequest) SetIndex(index string) {
	r.Mget = r.Index(index)
}

func (r *MGetRequest) SetIDs(ids ...string) {
	r.Mget = r.Ids(ids...)
}

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
