// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"log/slog"
	"strings"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/count"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/msearch"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/update"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/refresh"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic/aggregations"
	"github.com/immanent-tech/foragd/providers/elastic/query"
)

const (
	// ReqIDHeader is the value that will be used to assign a unique ID to an Elasticsearch API request (that can be
	// used to associate the API request with a web server request).
	ReqIDHeader = "X-Opaque-Id"
)

// RequestCommon represents the common methods for any Elasticsearch request,.
type RequestCommon[T any] interface {
	Header(key string, value string) T
}

// WithRequestID option sets the appropriate request ID header to the given value in the request.
func WithRequestID[T any, V RequestCommon[T]](id string) Option[V] {
	return func(t V) {
		if id != "" {
			t.Header(ReqIDHeader, id)
		}
	}
}

// RequestWithQuery represents any Elasticsearch request that can accept a query.
type RequestWithQuery[T any] interface {
	Query(query types.QueryVariant) T
}

// WithQueryOptions option applies the given query options to the request.
func WithQueryOptions[T any, R RequestWithQuery[T]](options ...query.Option) Option[R] {
	return func(req R) {
		if query := query.Build(options...); query != nil {
			req.Query(query)
		}
	}
}

// RequestWithAggregations represents any Elasticsearch request that can accept aggregations.
type RequestWithAggregations[T any] interface {
	Aggregations(aggs map[string]types.Aggregations) T
}

// WithAggregations adds the given aggregation definitions to the search.
func WithAggregations[T any, R RequestWithAggregations[T]](aggs aggregations.Aggs) Option[R] {
	return func(req R) {
		req.Aggregations(aggs)
	}
}

// RequestWithIndex represents any Elasticsearch request that can specify an index to operate on.
type RequestWithIndex[T any] interface {
	Index(index string) T
}

// WithIndex option specifies the index for the request to operate on.
func WithIndex[T any, V RequestWithIndex[T]](index string) Option[V] {
	return func(t V) {
		if index != "" {
			t.Index(index)
		}
	}
}

// RequestWithIDs represents any Elasticsearch request that can accept a list of document IDs.
type RequestWithIDs[T any] interface {
	Ids(id ...string) T
}

// WithIDs option specifies the document IDs to operate on.
func WithIDs[T any, V RequestWithIDs[T]](ids ...string) Option[V] {
	return func(v V) {
		v.Ids(ids...)
	}
}

// RequestWithSize represents any Elasticsearch request that allows specifying a size (or number of results).
type RequestWithSize[T any] interface {
	Size(size int) T
}

// WithSize option defines the number of results returned.
func WithSize[T any, V RequestWithSize[T]](size int) Option[V] {
	return func(v V) {
		v.Size(size)
	}
}

// RequestWithSearchAfter represents any Elasticsearch request that can accept a "search after" value (i.e.,
// pagination).
type RequestWithSearchAfter[T any] interface {
	SearchAfter(values ...types.FieldValueVariant) T
}

// WithSearchAfter option sets the sort value to fetch the next set of results. It can
// accept either a []types.FieldValue or a []byte (html-encoded
// []types.FieldValue).
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/paginate-search-results.html#search-after
func WithSearchAfter[T any, V RequestWithSearchAfter[T]](values ...types.FieldValueVariant) Option[V] {
	return func(req V) {
		if values == nil {
			return
		}
		req.SearchAfter(values...)
	}
}

// RequestWithSort represents any Elasticsearch request that can be sorted.
type RequestWithSort[T any] interface {
	Sort(sort ...types.SortCombinationsVariant) T
}

// WithSortOptions option adds the given sorting options to the search.
func WithSortOptions[T any, V RequestWithSort[T]](options ...types.SortCombinationsVariant) Option[V] {
	return func(req V) {
		req.Sort(options...)
	}
}

// CountOption is a functional option to apply to a count request.
type CountOption Option[*count.Count]

// SearchRequest represents a `_search` API request.
type SearchRequest interface {
	RequestCommon[*search.Search]
	RequestWithIndex[*search.Search]
	RequestWithQuery[*search.Search]
	RequestWithAggregations[*search.Search]
	RequestWithSize[*search.Search]
	RequestWithSearchAfter[*search.Search]
	RequestWithSort[*search.Search]
}

// NewSearchRequest creates a new search request with the given options.
func NewSearchRequest(api *elasticsearch.TypedClient, options ...Option[SearchRequest]) *search.Search {
	req := api.Search()

	for _, option := range options {
		option(req)
	}

	return req
}

// WithSearch option adds a search to a multisearch request.
func WithSearch(search *models.MultiSearchQuery) Option[MsearchRequest] {
	return func(req MsearchRequest) {
		hdr := types.NewMultisearchHeader()
		hdr.Index = append(hdr.Index, search.Index)
		searchBody := types.NewSearchRequestBody()
		if query := query.Build(search.Query); query != nil {
			searchBody.Query = query
		}
		switch {
		case strings.HasPrefix(search.Index, "feeds"):
			searchBody.Sort = newFeedSortCombinations(search.Sort)
		case strings.HasPrefix(search.Index, "items"):
			searchBody.Sort = newItemSortCombinations(search.Sort)
		default:
			opts := &types.SortOptions{
				Doc_: types.NewScoreSort(),
			}
			c := types.SortCombinations(opts)
			searchBody.Sort = []types.SortCombinations{c}
		}
		searchBody.Size = &search.Size

		err := req.AddSearch(*hdr, *searchBody)
		if err != nil {
			slog.Warn("error occurred", slog.Any("error", err))
		}
	}
}

// MsearchRequest represents an Elasticsearch _msearch request.
type MsearchRequest interface {
	RequestCommon[*msearch.Msearch]
	AddSearch(header types.MultisearchHeader, body types.SearchRequestBody) error
}

// NewMSearchRequest creates a new multisearch request with the given options.
func NewMSearchRequest(api *elasticsearch.TypedClient, options ...Option[MsearchRequest]) *msearch.Msearch {
	req := api.Msearch()

	for _, option := range options {
		option(req)
	}
	return req
}

// UpdateDocRequest wraps the doc update api endpoint.
type UpdateDocRequest interface {
	RequestCommon[*update.Update]
	DocAsUpsert(docasupsert bool) *update.Update
	Refresh(refresh refresh.Refresh) *update.Update
	RetryOnConflict(retryonconflict int) *update.Update
}

// NewUpdateDocRequest creates a new doc update request with the given options.
func NewUpdateDocRequest(api *elasticsearch.TypedClient, index, id string, doc any, options ...Option[UpdateDocRequest]) *update.Update {
	req := api.Update(index, id).Doc(doc)

	for _, option := range options {
		option(req)
	}

	return req
}

// WithRefresh sets the refresh value for the request.
//
// https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-update#operation-update-refresh
func WithRefresh(value string) Option[UpdateDocRequest] {
	return func(req UpdateDocRequest) {
		req.Refresh(refresh.Refresh{Name: value})
	}
}

// WithRetryOnConflict sets the number of retries the request will make if a version conflict is detected.
//
// https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-update#operation-update-retry_on_conflict
func WithRetryOnConflict(attempts int) Option[UpdateDocRequest] {
	return func(req UpdateDocRequest) {
		req.RetryOnConflict(attempts)
	}
}

// UpdateDocAsUpsert ensures that a doc update will act as an upsert if there is no existing doc.
//
// https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-update#operation-update-body-application-json-doc_as_upsert
func UpdateDocAsUpsert() Option[UpdateDocRequest] {
	return func(req UpdateDocRequest) {
		req.DocAsUpsert(true)
	}
}

// CountRequest wraps the count API endpoint.
type CountRequest interface {
	RequestCommon[*count.Count]
	RequestWithIndex[*count.Count]
	RequestWithQuery[*count.Count]
}

// NewCountRequest creates a new count request with the given options.
func NewCountRequest(api *elasticsearch.TypedClient, options ...Option[CountRequest]) *count.Count {
	req := api.Count()

	for _, option := range options {
		option(req)
	}

	return req
}

// FieldValue represents a value of a field.
type FieldValue struct {
	value any
}

// NewFieldValue converts any value into a FieldValue.
func NewFieldValue(value any) FieldValue {
	return FieldValue{value: value}
}

// FieldValueCaster is required to allow FieldValue to be used as an Elasticsearch  field value.
func (v FieldValue) FieldValueCaster() *types.FieldValue {
	switch data := v.value.(type) {
	case types.FieldValue:
		return &data
	default:
		fv := types.FieldValue(data)
		return &fv
	}
}
