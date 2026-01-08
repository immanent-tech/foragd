// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/elastic/go-elasticsearch/v9/typedapi/core/count"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/deletebyquery"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/get"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/mget"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/refresh"
	"github.com/go-chi/chi/v5/middleware"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/logging"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/results"
)

var ErrNotFound = errors.New("not found")

var (
	_ types.FieldValueVariant = (*paginationValue[types.FieldValue])(nil)
)

// Object represents any kind of object that has an ID. Effectively the object can be indexed in Elasticsearch.
type Object[T ~string] interface {
	GetID() T
}

// Option is a generic type for functional options.
type Option[T any] func(T)

// BulkAdd will create documents for the given list of objects. Responses are returned as a map of doc id to response.
// If the request itself fails, a non-nil error is returned.
func BulkAdd[T ~string, O Object[T]](
	ctx context.Context,
	index string,
	objects ...O,
) (map[T]*bulk.OperationResponse, error) {
	// Connect to elasticsearch (if not already connected).
	if err := Connect(); err != nil {
		return nil, fmt.Errorf("connect to elasticsearch: %w", err)
	}

	bulkOps, respCh := bulk.NewRequest(ctx, api.TypedClient)

	go func() {
		defer close(bulkOps)

		for object := range slices.Values(objects) {
			bulkOps <- bulk.NewOperation(object,
				bulk.SetDocID(string(object.GetID())),
				bulk.ToIndex(index),
			)
		}
	}()

	bulkOpResponse := <-respCh
	// If the request failed, return an error.
	if bulkOpResponse.Err != nil {
		return nil, fmt.Errorf("bulk add: %w", bulkOpResponse.Err)
	}
	// Create a map of responses by object id.
	responses := make(map[T]*bulk.OperationResponse)
	// Map responses to object id.
	for opResp := range slices.Values(bulkOpResponse.Responses) {
		if opResp.Id_ == nil {
			continue
		}
		if idx := slices.IndexFunc(objects, func(o O) bool {
			return string(o.GetID()) == *opResp.Id_
		}); idx != -1 {
			responses[objects[idx].GetID()] = opResp
		}
	}

	return responses, nil
}

// BulkUpdate will update documents for the given list of objects. Responses are returned as a map of doc id to response.
// If the request itself fails, a non-nil error is returned.
func BulkUpdate[T ~string, O Object[T]](
	ctx context.Context,
	index string,
	objects ...O,
) (map[T]*bulk.OperationResponse, error) {
	// Connect to elasticsearch (if not already connected).
	if err := Connect(); err != nil {
		return nil, fmt.Errorf("connect to elasticsearch: %w", err)
	}

	bulkOps, respCh := bulk.NewRequest(ctx, api.TypedClient)

	go func() {
		defer close(bulkOps)

		for object := range slices.Values(objects) {
			bulkOps <- bulk.NewOperation(object,
				bulk.AsOperationType(bulk.BulkUpdate),
				bulk.SetDocID(string(object.GetID())),
				bulk.ToIndex(index),
				bulk.Upsert(true),
			)
		}
	}()

	bulkOpResponse := <-respCh
	// If the request failed, return an error.
	if bulkOpResponse.Err != nil {
		return nil, fmt.Errorf("bulk update: %w", bulkOpResponse.Err)
	}
	// Create  a map of responses by object id.
	responses := make(map[T]*bulk.OperationResponse)
	// Map responses to object id.
	for opResp := range slices.Values(bulkOpResponse.Responses) {
		if opResp.Id_ == nil {
			continue
		}
		if idx := slices.IndexFunc(objects, func(o O) bool {
			return string(o.GetID()) == *opResp.Id_
		}); idx != -1 {
			responses[objects[idx].GetID()] = opResp
		}
	}

	return responses, nil
}

// Count will return the number of docs matching the given queries in the given index.
func Count(ctx context.Context, index string, queries ...query.Option) (int64, error) {
	// Connect to elasticsearch (if not already connected).
	if err := Connect(); err != nil {
		return 0, fmt.Errorf("connect to elasticsearch: %w", err)
	}

	resp, err := NewCountRequest(api.TypedClient,
		WithRequestID[*count.Count, CountRequest](middleware.GetReqID(ctx)),
		WithIndex[*count.Count, CountRequest](index),
		WithQueryOptions[*count.Count, CountRequest](queries...),
	).Do(ctx)
	if err != nil {
		return 0, fmt.Errorf("count: %w", err)
	}

	return resp.Count, nil
}

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
	resp, err := NewMGetRequest(api.TypedClient,
		WithRequestID[*mget.Mget, MgetRequest](middleware.GetReqID(ctx)),
		WithIndex[*mget.Mget, MgetRequest](index),
		WithIDs[*mget.Mget, MgetRequest](docIDs...),
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

	resp, err := NewGetRequest(api.TypedClient, index, string(id),
		WithRequestID[*get.Get, RequestCommon[*get.Get]](middleware.GetReqID(ctx)),
	).Do(ctx)
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
	updates map[string]any,
	options ...Option[UpdateDocRequest],
) error {
	// Connect to elasticsearch (if not already connected).
	if err := Connect(); err != nil {
		return fmt.Errorf("connect to elasticsearch: %w", err)
	}

	resp, err := NewUpdateDocRequest(api.TypedClient, index, string(id), updates, options...).Do(ctx)
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

// DeleteDocs performs a delete by query request on the given index to delete documents matching the given queries.
func DeleteDocs(ctx context.Context, index string, queries ...query.Option) error {
	// Connect to elasticsearch (if not already connected).
	if err := Connect(); err != nil {
		return fmt.Errorf("connect to elasticsearch: %w", err)
	}

	resp, err := NewDeleteByQueryRequest(
		api.TypedClient,
		index,
		WithRequestID[*deletebyquery.DeleteByQuery, RequestCommon[*deletebyquery.DeleteByQuery]](
			middleware.GetReqID(ctx),
		),
		WithQueryOptions[*deletebyquery.DeleteByQuery, RequestWithQuery[*deletebyquery.DeleteByQuery]](queries...),
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

// Search performs a _search request to find documents matching the given query.
func Search[O any](
	ctx context.Context,
	index string,
	query query.Option,
	count int,
	options ...Option[SearchRequest],
) ([]O, []types.FieldValue, error) {
	// Connect to elasticsearch (if not already connected).
	if err := Connect(); err != nil {
		return nil, nil, fmt.Errorf("connect to elasticsearch: %w", err)
	}

	defaultOptions := []Option[SearchRequest]{
		WithRequestID[*search.Search, SearchRequest](middleware.GetReqID(ctx)),
		WithIndex[*search.Search, SearchRequest](index),
		WithQueryOptions[*search.Search, SearchRequest](query),
		WithSize[*search.Search, SearchRequest](count),
	}
	defaultOptions = append(defaultOptions, options...)
	req := NewSearchRequest(defaultOptions...)

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("search: %w", err)
	}
	var warnings error
	var docs []O
	var newSearchAfter []types.FieldValue

	docs, newSearchAfter, warnings = results.ExtractSourceFromHits[O](resp.Hits.Hits)
	if warnings != nil {
		slogctx.FromCtx(ctx).WarnContext(ctx, "Some docs could not be extracted.",
			slog.Any("warnings", warnings))
	}

	return docs, newSearchAfter, nil
}

// SearchAll performs a paginated search request to retrieve *all* documents matching the given query. Unlike Search, it
// does not stop when the request hits count is reached.
func SearchAll[O any](
	ctx context.Context,
	index string,
	query query.Option,
	paginationSize int,
	options ...Option[SearchRequest],
) ([]O, error) {
	// Connect to elasticsearch (if not already connected).
	if err := Connect(); err != nil {
		return nil, fmt.Errorf("connect to elasticsearch: %w", err)
	}

	if paginationSize == 0 {
		paginationSize = 1000
	}
	allResults := make([]O, 0)
	var searchAfter []types.FieldValueVariant

	// Loop until we've paginated through all results.
	var loops int
	for {
		searchOpts := []Option[SearchRequest]{
			WithRequestID[*search.Search, SearchRequest](middleware.GetReqID(ctx)),
			WithIndex[*search.Search, SearchRequest](index),
			WithQueryOptions[*search.Search, SearchRequest](query),
			WithSortOptions[*search.Search, SearchRequest](&types.SortOptions{Doc_: types.NewScoreSort()}),
			WithSearchAfter[*search.Search, SearchRequest](searchAfter...),
			WithTrackTotalHits(false),
		}
		searchOpts = append(searchOpts, options...)
		resultsPage, nextSearchAfter, err := Search[O](ctx, index, query, paginationSize, searchOpts...)
		if err != nil {
			return nil, fmt.Errorf("search all: %w", err)
		}
		pagination, err := EncodePagination[string](nextSearchAfter)
		if err != nil {
			return nil, fmt.Errorf("search all: encode pagination: %w", err)
		}
		searchAfter, err = DecodePagination(pagination)
		if err != nil {
			return nil, fmt.Errorf("search all: decode pagination: %w", err)
		}

		allResults = append(allResults, resultsPage...)
		// Stop if the number of hits is less than the search size (i.e., last set of hits).
		if len(resultsPage) < paginationSize {
			break
		}
		loops++
	}
	slogctx.FromCtx(ctx).Log(ctx, logging.LevelTrace, "Paginated search finished.",
		slog.Int("loops", loops),
	)
	return allResults, nil
}

// // MultiSearch performs an msearch request.
// func MultiSearch(ctx context.Context, api *elasticsearch.TypedClient, searches ...*models.MultiSearchQuery) (results.MSearchResults, error) {
// 	// subscriptionsIndex, err := FeedsReadIndexFromCtx(ctx)
// 	// if err != nil {
// 	// 	return nil, errors.Join(ErrUpdateFailed, ErrFetchCtx)
// 	// }
// 	// itemsIndex, err := ItemsReadIndexFromCtx(ctx)
// 	// if err != nil {
// 	// 	return nil, fmt.Errorf("unable to perform multi-search: %w", err)
// 	// }

// 	options := make([]Option[MsearchRequest], 0, len(searches)+1)
// 	options = append(options, WithRequestID[*msearch.Msearch, MsearchRequest](middleware.GetReqID(ctx)))
// 	for search := range slices.Values(searches) {
// 		options = append(options, WithSearch(search))
// 	}

// 	req := NewMSearchRequest(api, options...)
// 	resp, err := req.Do(ctx)
// 	if err != nil {
// 		return nil, fmt.Errorf("multisearch: msearch request failed: %w", err)
// 	}

// 	results := make(map[string]*types.MultiSearchItem)
// 	for idx, search := range searches {
// 		if result, ok := resp.Responses[idx].(*types.MultiSearchItem); ok {
// 			results[search.Name] = result
// 		}
// 	}

// 	return results, nil
// }
