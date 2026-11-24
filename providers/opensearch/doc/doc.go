/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package doc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sync"

	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/opensearch"
	"github.com/immanent-tech/foragd/providers/opensearch/search"
)

type Client interface {
	DeleteByQuery(ctx context.Context, req *opensearchapi.DeleteByQueryReq) (*opensearchapi.DeleteByQueryResp, error)
}

// Get will retrieve the document with the given ID from the given index.
//
// https://docs.opensearch.org/latest/api-reference/document-apis/get-documents/
func Get[T ~string, O opensearch.Doc[T]](
	ctx context.Context,
	client opensearchapi.DocumentClient,
	index string,
	id T,
) (O, error) {
	var doc O
	req := opensearchapi.GetReq{
		Index:  index,
		ID:     string(id),
		Header: opensearch.NewRequestHeaders(ctx),
	}

	resp, err := client.Get(ctx, req)
	if err != nil {
		// Check for 404.
		if resp != nil && resp.Inspect().Response.StatusCode == http.StatusNotFound {
			return doc, errors.Join(models.ErrNotFound, err)
		}
		// Handle all other errors.
		for _, sub := range opensearchapi.Errors(err) {
			switch e := sub.(type) {
			case *opensearchapi.ShardFailureError:
				slogctx.Warn(ctx, "Partial failure for doc request.",
					slog.Int("failed_shards", e.FailedShards),
					slog.Int("total_shards", e.TotalShards),
					slog.String("operation", e.Operation),
				)
			default:
				return doc, opensearch.HandleError("get doc", resp, err)
			}
		}
	}
	// Check response status.
	if !resp.Found {
		return doc, errors.Join(models.ErrNotFound, err)
	}

	doc, err = opensearch.ExtractSource[O](resp.Source)
	if err != nil {
		return doc, models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("extract source: %w", err))
	}

	return doc, nil
}

// GetDocs will retrieve the documents with the given IDs from the given index.
//
// https://docs.opensearch.org/latest/api-reference/document-apis/multi-get/
func GetDocs[T ~string, O opensearch.Doc[T]](
	ctx context.Context,
	client opensearchapi.DocumentClient,
	index string,
	ids ...T,
) ([]O, error) {
	allDocs := make([]opensearchapi.MGetOperation, 0, len(ids))
	for id := range slices.Values(ids) {
		allDocs = append(allDocs, opensearchapi.MGetOperation{
			ID: string(id),
		})
	}
	req := opensearchapi.MGetReq{
		Index: index,
		Body: &opensearchapi.MGetBody{
			Docs: allDocs,
		},
		Header: opensearch.NewRequestHeaders(ctx),
	}
	resp, err := client.MGet(ctx, req)
	if err != nil {
		for _, sub := range opensearchapi.Errors(err) {
			switch e := sub.(type) {
			case *opensearchapi.ShardFailureError:
				slogctx.Warn(ctx, "Partial failure for doc request.",
					slog.Int("failed_shards", e.FailedShards),
					slog.Int("total_shards", e.TotalShards),
					slog.String("operation", e.Operation),
				)
			default:
				return nil, opensearch.HandleError("get doc", resp, err)
			}
		}
	}

	var errs []error
	docs := make([]O, 0, len(resp.Docs))
	for result := range slices.Values(resp.Docs) {
		g, err := result.GetResult()
		if err != nil {
			errs = append(errs, fmt.Errorf("get result: %w", err))
			continue
		}
		v, err := opensearch.ExtractSource[O](g.Source)
		if err != nil {
			errs = append(errs, err)
		}
		docs = append(docs, v)
	}

	return docs, nil
}

// Index will create the given document, with given id, in the given index. If a document with the ID already exists, it
// will be overridden.
//
// https://docs.opensearch.org/latest/api-reference/document-apis/index-document/
func Index[T ~string, O opensearch.Doc[T]](
	ctx context.Context,
	client opensearchapi.DocumentClient,
	index string,
	doc O,
	options ...IndexOption,
) error {
	data, err := marshalDoc(doc)
	if err != nil {
		return models.NewAPIError(http.StatusUnprocessableEntity, err)
	}

	req := opensearchapi.IndexReq{
		Index:  index,
		ID:     string(doc.GetID()),
		Header: opensearch.NewRequestHeaders(ctx),
		Body:   data,
	}

	for option := range slices.Values(options) {
		option(&req)
	}

	resp, err := client.Index(ctx, req)
	if err != nil {
		for _, sub := range opensearchapi.Errors(err) {
			switch e := sub.(type) {
			case *opensearchapi.ShardFailureError:
				slogctx.Warn(ctx, "Partial failure for doc request.",
					slog.Int("failed_shards", e.FailedShards),
					slog.Int("total_shards", e.TotalShards),
					slog.String("operation", e.Operation),
				)
			default:
				return opensearch.HandleError("index doc", resp, err)
			}
		}
	}

	return nil
}

// IndexOption is a functional option applied to an [Index] request.
type IndexOption func(*opensearchapi.IndexReq)

// WithIndexRefresh option sets the refresh value for the index operation.
func WithIndexRefresh(value opensearchapi.Refresh) IndexOption {
	return func(r *opensearchapi.IndexReq) {
		if r.Params == nil {
			r.Params = &opensearchapi.IndexParams{}
		}
		r.Params.Refresh = value
	}
}

// Create will create the given document, with given id, in the given index. If a document with the given ID already
// exists, it will return a 409 error. Use [Index] to "upsert" a doc where it might already exists.
//
// https://docs.opensearch.org/latest/api-reference/document-apis/index-document/
func Create[T ~string, O opensearch.Doc[T]](
	ctx context.Context,
	client opensearchapi.DocumentClient,
	index string,
	doc O,
) error {
	data, err := marshalDoc(doc)
	if err != nil {
		return models.NewAPIError(http.StatusUnprocessableEntity, err)
	}

	req := opensearchapi.CreateReq{
		Index:  index,
		ID:     string(doc.GetID()),
		Body:   data,
		Header: opensearch.NewRequestHeaders(ctx),
	}

	resp, err := client.Create(ctx, req)
	if err != nil {
		for _, sub := range opensearchapi.Errors(err) {
			switch e := sub.(type) {
			case *opensearchapi.ShardFailureError:
				slogctx.Warn(ctx, "Partial failure for doc request.",
					slog.Int("failed_shards", e.FailedShards),
					slog.Int("total_shards", e.TotalShards),
					slog.String("operation", e.Operation),
				)
			default:
				return opensearch.HandleError("create doc", resp, err)
			}
		}
	}

	return nil
}

// Update will apply the updates to the doc with the given id.
//
// https://docs.opensearch.org/latest/api-reference/document-apis/update-document/
func Update[T ~string](
	ctx context.Context,
	client opensearchapi.DocumentClient,
	index string,
	id T,
	doc any,
	options ...UpdateOption,
) error {
	// Set any specified options for the request.
	body := &opensearchapi.UpdateBody{}
	for option := range slices.Values(options) {
		option(body)
	}

	// Encode the updates.
	data, err := marshalDoc(doc)
	if err != nil {
		return models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("encode updates: %w", err))
	}
	body.Doc = data.Bytes()

	// Create the request.
	req := opensearchapi.UpdateReq{
		Index:  index,
		Header: opensearch.NewRequestHeaders(ctx),
		Body:   body,
		ID:     string(id),
	}

	// Perform the update.
	resp, err := client.Update(ctx, req)
	if err != nil {
		for _, sub := range opensearchapi.Errors(err) {
			switch e := sub.(type) {
			case *opensearchapi.ShardFailureError:
				slogctx.Warn(ctx, "Partial failure for doc request.",
					slog.Int("failed_shards", e.FailedShards),
					slog.Int("total_shards", e.TotalShards),
					slog.String("operation", e.Operation),
				)
			default:
				return opensearch.HandleError("update doc", resp, err)
			}
		}
	}

	return nil
}

// UpdateOption is a functional option to apply to an [Update] request.
type UpdateOption func(*opensearchapi.UpdateBody)

// WithDocAsUpsert will add the doc as a new document if not found. Otherwise, the doc will be updated.
func WithDocAsUpsert(value bool) UpdateOption {
	return func(b *opensearchapi.UpdateBody) {
		b.DocAsUpsert = &value
	}
}

// WithUpdateDetectNoop option if set to `false` to disable setting `result` in the response to `noop` if no change to
// the document occurred.
func WithUpdateDetectNoop(value bool) UpdateOption {
	return func(b *opensearchapi.UpdateBody) {
		b.DetectNoop = &value
	}
}

// Delete performs a document deletion request.
//
// https://docs.opensearch.org/latest/api-reference/document-apis/delete-document/
func Delete[T ~string](
	ctx context.Context,
	client opensearchapi.DocumentClient,
	index string,
	id T,
	options ...DeleteOption,
) error {
	req := opensearchapi.DeleteReq{
		Index:  index,
		ID:     string(id),
		Header: opensearch.NewRequestHeaders(ctx),
		Params: &opensearchapi.DeleteParams{},
	}

	// Set any additional optional parameters.
	for option := range slices.Values(options) {
		option(req.Params)
	}

	resp, err := client.Delete(ctx, req)
	if err != nil {
		for _, sub := range opensearchapi.Errors(err) {
			switch e := sub.(type) {
			case *opensearchapi.ShardFailureError:
				slogctx.Warn(ctx, "Partial failure for doc request.",
					slog.Int("failed_shards", e.FailedShards),
					slog.Int("total_shards", e.TotalShards),
					slog.String("operation", e.Operation),
				)
			default:
				return opensearch.HandleError("delete doc", resp, err)
			}
		}
	}

	return nil
}

// DeleteOption is a functional option applied to a [Delete] request.
type DeleteOption func(*opensearchapi.DeleteParams)

// WithDeleteSeqNo option ensures the deletion will only happen if the document has the given sequence number.
func WithDeleteSeqNo(num int) DeleteOption {
	return func(r *opensearchapi.DeleteParams) {
		r.IfSeqNo = &num
	}
}

// WithDeletePrimaryTerm option ensures the deletion will only happen if the document has the given primary term.
func WithDeletePrimaryTerm(term int) DeleteOption {
	return func(r *opensearchapi.DeleteParams) {
		r.IfPrimaryTerm = &term
	}
}

// WithDeleteRefresh option sets the refresh value for the delete operation.
func WithDeleteRefresh(refresh opensearchapi.Refresh) DeleteOption {
	return func(r *opensearchapi.DeleteParams) {
		r.Refresh = refresh
	}
}

// DeleteByQuery performs a delete by query request on the given index with the given options.
//
// https://docs.opensearch.org/latest/api-reference/document-apis/delete-by-query/
func DeleteByQuery(ctx context.Context, client Client, index string, options ...DeleteByQueryOption) error {
	// Set any specified options for the request.
	body := &opensearchapi.DeleteByQueryBody{}
	for option := range slices.Values(options) {
		option(body)
	}

	req := &opensearchapi.DeleteByQueryReq{
		Indices: []string{index},
		Header:  opensearch.NewRequestHeaders(ctx),
		Body:    body,
	}

	resp, err := client.DeleteByQuery(ctx, req)
	if err != nil {
		for _, sub := range opensearchapi.Errors(err) {
			switch e := sub.(type) {
			case *opensearchapi.ShardFailureError:
				slogctx.Warn(ctx, "Partial failure for doc request.",
					slog.Int("failed_shards", e.FailedShards),
					slog.Int("total_shards", e.TotalShards),
					slog.String("operation", e.Operation),
				)
			default:
				return opensearch.HandleError("delete by query", resp, err)
			}
		}
	}

	return nil
}

// DeleteByQueryOption is a functional option applied to a [DeleteByQuery] request.
type DeleteByQueryOption func(*opensearchapi.DeleteByQueryBody)

// WithDeleteByQuery option specifies the query used to select documents to delete.
func WithDeleteByQuery(options ...search.QueryOption) DeleteByQueryOption {
	return func(req *opensearchapi.DeleteByQueryBody) {
		query := &opensearchapi.CommonQueryDSLQueryContainer{}
		for option := range slices.Values(options) {
			option(query)
		}
		req.Query = query
	}
}

// WithDeleteMaxDocs options specifies the maximum number of documents to delete.
func WithDeleteMaxDocs(max int) DeleteByQueryOption {
	return func(req *opensearchapi.DeleteByQueryBody) {
		req.MaxDocs = &max
	}
}

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

func marshalDoc(doc any) (*bytes.Buffer, error) {
	bodyBuf, ok := bufPool.Get().(*bytes.Buffer)
	if !ok {
		return nil, errors.New("get buffer failed")
	}
	bodyBuf.Reset()
	defer bufPool.Put(bodyBuf)

	if err := json.NewEncoder(bodyBuf).Encode(doc); err != nil {
		return nil, fmt.Errorf("encode doc: %w", err)
	}

	return bodyBuf, nil
}
