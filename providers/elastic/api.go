// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//nolint:dupl
package elastic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/count"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/deletebyquery"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/get"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/mget"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/msearch"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/refresh"
	"github.com/go-chi/chi/v5/middleware"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/logging"
	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic/aggregations"
	"github.com/joshuar/go-feed-me/providers/elastic/bulk"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/providers/elastic/results"
)

var _ types.FieldValueVariant = (*paginationValue[types.FieldValue])(nil)

// API is an object that provides access to the Elasticsearch API.
type API struct {
	*elasticsearch.TypedClient
}

// // GetAPI returns the raw API object.
func (a *API) GetAPI() *elasticsearch.TypedClient {
	return a.TypedClient
}

// GetFeeds retrieves the feeds with the given IDs.
func (a *API) GetFeeds(ctx context.Context, ids ...models.FeedID) (models.Feeds, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, ErrFetchCtx
	}

	feeds, err := GetDocs[models.FeedID, *models.Feed](ctx, a.GetAPI(), index, ids...)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Some subscriptions could not be extracted from docs.",
			slog.Any("warnings", err))
	}
	return feeds, nil
}

// SearchFeeds will search the feeds index for feed matching the given query. Count, sort and pagination values are
// optional.
func (e *API) SearchFeeds(ctx context.Context, query query.Option, count int, sort *models.Sort, pagination *models.Pagination) (models.Feeds, models.Pagination, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, "", errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	searchAfter, err := decodePagination(pagination)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrSearchFailed, err)
	}

	// Perform search.
	feeds, newSearchAfter, err := Search[*models.Feed](ctx, e.GetAPI(), index, query, count,
		WithSortOptions[*search.Search, SearchRequest](newFeedSortOptions(sort)...),
		WithSearchAfter[*search.Search, SearchRequest](searchAfter...),
	)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
	}
	// Parse search after into pagination.
	if pagination != nil {
		*pagination, err = encodePagination(newSearchAfter)
		if err != nil {
			return nil, "", errors.Join(ErrSearchFailed, err)
		}
		return feeds, *pagination, nil
	}

	return feeds, "", nil
}

// SearchItems will search the items index for items matching the given query. Count, sort and pagination values are
// optional.
func (e *API) SearchItems(ctx context.Context, query query.Option, count int, sort *models.Sort, pagination *models.Pagination) (models.Items, models.Pagination, error) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, "", errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	searchAfter, err := decodePagination(pagination)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrSearchFailed, err)
	}
	// Perform search.
	items, newSearchAfter, err := Search[*models.Item](ctx, e.GetAPI(), index, query, count,
		WithSortOptions[*search.Search, SearchRequest](newItemSortOptions(sort)...),
		WithSearchAfter[*search.Search, SearchRequest](searchAfter...),
	)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
	}
	// Parse last search after value into pagination.
	newPagination, err := encodePagination(newSearchAfter)
	if err != nil {
		return nil, "", errors.Join(ErrSearchFailed, err)
	}
	return items, newPagination, nil
}

// ItemsAggregation performs a search aggregation (i.e., only aggregations returned) on feed items with the given query
// options. It returns the raw search response.
func (e *API) ItemsAggregation(ctx context.Context, query query.Option, aggregations ...aggregations.Aggregation) (*search.Response, *models.Response) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, ParseError(ErrFetchCtx)
	}

	req := NewSearchRequest(e.GetAPI(),
		WithRequestID[*search.Search, SearchRequest](middleware.GetReqID(ctx)),
		WithIndex[*search.Search, SearchRequest](index),
		WithQueryOptions[*search.Search, SearchRequest](query),
		WithSize[*search.Search, SearchRequest](0),
		WithSortOptions[*search.Search, SearchRequest](&DocSorting{}),
		WithAggregations[*search.Search, SearchRequest](aggregations...),
	)
	resp, err := req.Do(ctx)
	if err != nil {
		return nil, ParseError(err)
	}

	return resp, nil
}

// AddItems will bulk index the given items.
func (e *API) AddItems(ctx context.Context, items ...*models.Item) (map[models.ItemID]*bulk.OperationResponse, error) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, ErrFetchCtx
	}
	return BulkAdd(ctx, e, index, items...)
}

// BulkAdd will create documents for the given list of objects. Responses are returned as a map of doc id to response.
// If the request itself fails, a non-nil error is returned.
func BulkAdd[T ~string, O Object[T]](ctx context.Context, api *API, index string, objects ...O) (map[T]*bulk.OperationResponse, error) {
	bulkOps, respCh := bulk.NewRequest(ctx, api)

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
		return nil, fmt.Errorf("bulk operation failed: %w", bulkOpResponse.Err)
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

// BulkAdd will create documents for the given list of objects. Responses are returned as a map of doc id to response.
// If the request itself fails, a non-nil error is returned.
func BulkUpdate[T ~string, O Object[T]](ctx context.Context, api *API, index string, objects ...O) (map[T]*bulk.OperationResponse, error) {
	bulkOps, respCh := bulk.NewRequest(ctx, api)

	go func() {
		defer close(bulkOps)

		for object := range slices.Values(objects) {
			bulkOps <- bulk.NewOperation(object,
				bulk.AsOperationType(bulk.BulkUpdate),
				bulk.SetDocID(string(object.GetID())),
				bulk.ToIndex(index),
			)
		}
	}()

	bulkOpResponse := <-respCh
	// If the request failed, return an error.
	if bulkOpResponse.Err != nil {
		return nil, fmt.Errorf("%w: %w", ErrAPIRequestFailed, bulkOpResponse.Err)
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

// Exists checks if the document with the given id exists in the given index.
func Exists[T ~string](ctx context.Context, api *elasticsearch.TypedClient, index string, id T) (bool, error) {
	found, err := api.Exists(index, string(id)).
		Header(ReqIDHeader, middleware.GetReqID(ctx)).
		Do(ctx)
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
	}
	return found, nil
}

// Count will return the number of docs matching the given queries in the given index.
func Count(ctx context.Context, api *elasticsearch.TypedClient, index string, queries ...query.Option) (int64, error) {
	resp, err := NewCountRequest(api,
		WithRequestID[*count.Count, CountRequest](middleware.GetReqID(ctx)),
		WithIndex[*count.Count, CountRequest](index),
		WithQueryOptions[*count.Count, CountRequest](queries...),
	).Do(ctx)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
	}

	return resp.Count, nil
}

// GetDocs performs an `_mget` request to fetch the documents from the given index with the given ids. A non-nil error
// is returned on a failure.
func GetDocs[T ~string, O any](ctx context.Context, api *elasticsearch.TypedClient, index string, ids ...T) ([]O, error) {
	docIDs := make([]string, 0, len(ids))
	for id := range slices.Values(ids) {
		docIDs = append(docIDs, string(id))
	}

	resp, err := NewMGetRequest(api,
		WithRequestID[*mget.Mget, MgetRequest](middleware.GetReqID(ctx)),
		WithIndex[*mget.Mget, MgetRequest](index),
		WithIDs[*mget.Mget, MgetRequest](docIDs...),
	).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("get docs failed: %w", err)
	}
	objects, warnings := results.ExtractSourceFromDocs[O](resp.Docs)
	if warnings != nil {
		slogctx.FromCtx(ctx).Warn("Some docs could not be extracted.",
			slog.Any("warnings", warnings))
	}
	return objects, nil
}

// GetDoc retrieves the doc with the given id from the given index. A non-nil error is returned on a failure.
func GetDoc[T ~string, O any](ctx context.Context, api *elasticsearch.TypedClient, index string, id T) (O, error) {
	var doc O
	resp, err := NewGetRequest(api, index, string(id),
		WithRequestID[*get.Get, RequestCommon[*get.Get]](middleware.GetReqID(ctx)),
	).Do(ctx)
	if err != nil {
		return doc, fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
	}
	doc, err = results.ExtractSource[O](resp.Source_)
	if err != nil {
		return doc, fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
	}

	return doc, nil
}

// CreateDoc will create the given document, with given id, in the given index.
func CreateDoc[T ~string, O any](ctx context.Context, api *elasticsearch.TypedClient, index string, id T, doc O) error {
	resp, err := api.Create(index, string(id)).
		Document(doc).
		Header(ReqIDHeader, middleware.GetReqID(ctx)).
		Refresh(refresh.True).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
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
func UpdateDoc[T ~string](ctx context.Context, api *elasticsearch.TypedClient, index string, id T, updates map[string]any, options ...Option[UpdateDocRequest]) error {
	baseUpdates := map[string]any{
		"updated_at": time.Now().UTC(),
	}
	if updates != nil {
		// Add the updated_at field.
		maps.Copy(updates, baseUpdates)
	} else {
		// Just update the updated_at field (i.e., `touch` the document).
		updates = baseUpdates
	}

	// Update the user in the store with the new list of read items.
	resp, err := NewUpdateDocRequest(api, index, string(id), updates, options...).Do(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
	}
	if resp != nil {
		slogctx.FromCtx(ctx).Log(ctx, logging.LevelTrace, "Updated document.",
			slog.String("id", resp.Id_),
			slog.String("result", resp.Result.String()),
		)
	}

	return nil
}

// DeleteDoc deletes the document with the given id from the given index.
func DeleteDoc[T ~string](ctx context.Context, api *elasticsearch.TypedClient, index string, id T) error {
	resp, err := api.Delete(index, string(id)).
		Header(ReqIDHeader, middleware.GetReqID(ctx)).
		Refresh(refresh.True).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
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
func DeleteDocs(ctx context.Context, api *elasticsearch.TypedClient, index string, queries ...query.Option) error {
	resp, err := NewDeleteByQueryRequest(api, index,
		WithRequestID[*deletebyquery.DeleteByQuery, RequestCommon[*deletebyquery.DeleteByQuery]](middleware.GetReqID(ctx)),
		WithQueryOptions[*deletebyquery.DeleteByQuery, RequestWithQuery[*deletebyquery.DeleteByQuery]](queries...),
	).Do(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
	}
	if resp != nil {
		slogctx.FromCtx(ctx).Log(ctx, logging.LevelTrace, "Delete documents.",
			slog.Int64("count", *resp.Deleted),
		)
	}
	return nil
}

// Search performs a _search request to find documents matching the given query.
func Search[O any](ctx context.Context, api *elasticsearch.TypedClient, index string, query query.Option, count int, options ...Option[SearchRequest]) ([]O, []types.FieldValue, error) {
	defaultOptions := []Option[SearchRequest]{
		WithRequestID[*search.Search, SearchRequest](middleware.GetReqID(ctx)),
		WithIndex[*search.Search, SearchRequest](index),
		WithQueryOptions[*search.Search, SearchRequest](query),
		WithSize[*search.Search, SearchRequest](count),
	}
	defaultOptions = append(defaultOptions, options...)
	req := NewSearchRequest(api, defaultOptions...)
	// spew.Dump(req.HttpRequest(ctx))
	resp, err := req.Do(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("search request failed: %w", err)
	}

	var warnings error
	var docs []O
	var newSearchAfter []types.FieldValue

	docs, newSearchAfter, warnings = results.ExtractSourceFromHits[O](resp.Hits.Hits)
	if warnings != nil {
		slogctx.FromCtx(ctx).Warn("Some docs could not be extracted.",
			slog.Any("warnings", warnings))
	}

	return docs, newSearchAfter, nil
}

// SearchAll performs a paginated search request to retrieve *all* documents matching the given query. Unlike Search, it
// does not stop when the request hits count is reached.
func SearchAll[O any](ctx context.Context, api *elasticsearch.TypedClient, index string, query query.Option, paginationSize int, options ...Option[SearchRequest]) ([]O, error) {
	if paginationSize == 0 {
		paginationSize = 1000
	}
	allResults := make([]O, 0)
	var searchAfter []types.FieldValueVariant

	// Loop until we've paginated through all results.
	var loops int
	for {
		sortOptions := &DocSorting{}
		resultsPage, nextSearchAfter, err := Search[O](ctx, api, index, query, paginationSize,
			WithSortOptions[*search.Search, SearchRequest](sortOptions),
			WithSearchAfter[*search.Search, SearchRequest](searchAfter...),
		)
		if err != nil {
			return nil, fmt.Errorf("search request failed: %w", err)
		}
		pagination, err := encodePagination(nextSearchAfter)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrSearchFailed, err)
		}
		searchAfter, err = decodePagination(&pagination)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrSearchFailed, err)
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

func MultiSearch(ctx context.Context, api *elasticsearch.TypedClient, searches ...*query.MsearchSearch) (results.MSearchResults, error) {
	subscriptionsIndex := FeedsIndexFromCtx(ctx)
	if subscriptionsIndex == "" {
		return nil, errors.Join(ErrUpdateFailed, ErrFetchCtx)
	}
	itemsIndex := ItemsIndexFromCtx(ctx)
	if itemsIndex == "" {
		return nil, errors.Join(ErrUpdateFailed, ErrFetchCtx)
	}

	var options []Option[MsearchRequest]
	options = append(options, WithRequestID[*msearch.Msearch, MsearchRequest](middleware.GetReqID(ctx)))
	for search := range slices.Values(searches) {
		options = append(options, WithSearch(search))
	}

	req := NewMSearchRequest(api, options...)
	resp, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrReqFailed, err)
	}

	results := make(map[string]*types.MultiSearchItem)
	for idx, search := range searches {
		if result, ok := resp.Responses[idx].(*types.MultiSearchItem); ok {
			results[search.Name] = result
		}
	}

	return results, nil
}

func ParseError(err error) *models.Response {
	var esErr *types.ElasticsearchError
	if errors.As(err, &esErr) {
		return models.NewResponse(
			models.WithResponseStatusCode(esErr.Status),
			models.WithResponseError(esErr),
		)
	}
	return models.NewResponse(
		models.WithResponseStatusCode(http.StatusInternalServerError),
		models.WithResponseError(errors.New("unknown elastic error")),
	)
}

// paginationValue is a value that can be used as a sort value as a search after option.
type paginationValue[T types.FieldValue] struct {
	value T
}

func newPaginationValue[T any](value T) *paginationValue[T] {
	return &paginationValue[T]{value: value}
}

func (v *paginationValue[T]) FieldValueCaster() *types.FieldValue {
	casted := types.FieldValue(v)
	return &casted
}

func (v *paginationValue[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

// encodePagination will take sort values returned from a query, marshal them to
// JSON, then HTML-escape the string into a models.Pagination object, which is
// safe for use in API query parameters.
func encodePagination(sortValues []types.FieldValue) (models.Pagination, error) {
	if len(sortValues) == 0 {
		return "", nil
	}
	// Marshal sort values into json.
	data, err := json.Marshal(sortValues)
	if err != nil {
		return "", fmt.Errorf("could not encode pagination values: %w", err)
	}
	// Return as HTML encoded string.
	return url.QueryEscape(string(data)), nil
}

// decodePagination will take a models.Pagination object, HTML-unescape the
// string then unmarshal it back into sort values.
func decodePagination(pagination *models.Pagination) ([]types.FieldValueVariant, error) {
	if pagination == nil {
		return nil, nil
	}
	if *pagination == "" {
		return nil, nil
	}
	// Unescape HTML encoded data.
	data, err := url.QueryUnescape(*pagination)
	if err != nil {
		return nil, fmt.Errorf("could not decode pagination values: %w", err)
	}
	// Unmarshal sort values.
	var values []any
	err = json.Unmarshal([]byte(data), &values)
	if err != nil {
		return nil, fmt.Errorf("could not decode pagination values: %w", err)
	}
	casted := make([]types.FieldValueVariant, 0, len(values))
	for v := range slices.Values(values) {
		switch r := v.(type) {
		case string:
			casted = append(casted, newPaginationValue(r))
		case int64:
			casted = append(casted, newPaginationValue(r))
		case float64:
			casted = append(casted, newPaginationValue(r))
		default:
			casted = nil
		}
	}

	// Return sort values.
	return casted, nil
}
