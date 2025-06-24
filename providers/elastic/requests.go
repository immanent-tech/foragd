// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"log/slog"

	"github.com/elastic/go-elasticsearch/v8/typedapi"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/count"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/msearch"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/update"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/sortorder"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic/aggregations"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
)

const (
	ReqIDHeader = "X-Opaque-Id"
)

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

type RequestWithQuery[T any] interface {
	Query(query *types.Query) T
}

// WithQueryOptions option applies the given query options to the request.
func WithQueryOptions[T any, R RequestWithQuery[T]](options ...query.Option) Option[R] {
	return func(req R) {
		if query := query.Build(options...); query != nil {
			req.Query(query)
		}
	}
}

type RequestWithAggregations[T any] interface {
	Aggregations(aggs map[string]types.Aggregations) T
}

// WithAggregations adds the given aggregation definitions to the search.
func WithAggregations[T any, R RequestWithAggregations[T]](definitions ...aggregations.Aggregation) Option[R] {
	return func(req R) {
		aggregations := make(map[string]types.Aggregations)

		for _, definition := range definitions {
			aggregations[definition.Name] = definition.Definition
		}

		req.Aggregations(aggregations)
	}
}

type RequestWithIndex[T any] interface {
	Index(index string) T
}

// WithRequestID option sets the appropriate request ID header to the given value in the request.
func WithIndex[T any, V RequestWithIndex[T]](index string) Option[V] {
	return func(t V) {
		if index != "" {
			t.Index(index)
		}
	}
}

type RequestWithIDs[T any] interface {
	Ids(id ...string) T
}

func WithIDs[T any, V RequestWithIDs[T]](ids ...string) Option[V] {
	return func(v V) {
		v.Ids(ids...)
	}
}

type RequestWithSize[T any] interface {
	Size(size int) T
}

// WithSearchSize defines the number of results returned.
func WithSize[T any, V RequestWithSize[T]](size int) Option[V] {
	return func(v V) {
		v.Size(size)
	}
}

type RequestWithSearchAfter[T any] interface {
	SearchAfter(values ...types.FieldValue) T
}

// WithSearchAfter sets the sort value to fetch the next set of results. It can
// accept either a []types.FieldValue or a []byte (html-encoded
// []types.FieldValue).
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/paginate-search-results.html#search-after
func WithSearchAfter[T any, V RequestWithSearchAfter[T]](value []types.FieldValue) Option[V] {
	return func(req V) {
		if value == nil {
			return
		}
		req.SearchAfter(value...)
	}
}

type RequestWithSort[T any] interface {
	Sort(sort ...types.SortCombinations) T
}

// WithSortOptions adds the given sorting options to the search.
func WithSortOptions[T any, V RequestWithSort[T]](options ...types.SortCombinations) Option[V] {
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
func NewSearchRequest(api *typedapi.API, options ...Option[SearchRequest]) *search.Search {
	req := api.Search()

	for _, option := range options {
		option(req)
	}

	return req
}

// query *types.Query, sort []types.SortCombinations
func WithSearch(search *query.MsearchSearch) Option[MsearchRequest] {
	return func(req MsearchRequest) {
		if search == nil {
			return
		}

		hdr := types.NewMultisearchHeader()
		hdr.Index = append(hdr.Index, search.Index)

		searchBody := types.NewMultisearchBody()
		searchBody.Query = search.Query
		searchBody.Sort = search.Sort

		err := req.AddSearch(*hdr, *searchBody)
		if err != nil {
			slog.Warn("error occurred", slog.Any("error", err))
		}
	}
}

type MsearchRequest interface {
	RequestCommon[*msearch.Msearch]
	AddSearch(header types.MultisearchHeader, body types.MultisearchBody) error
}

func NewMSearchRequest(api *typedapi.API, options ...Option[MsearchRequest]) *msearch.Msearch {
	req := api.Msearch()

	for _, option := range options {
		option(req)
	}

	return req
}

// UpdateDocRequest wraps the doc update api.
type UpdateDocRequest interface {
	RequestCommon[*update.Update]
	DocAsUpsert(docasupsert bool) *update.Update
}

// NewUpdateDocRequest creates a new doc update request with the given options.
func NewUpdateDocRequest(api *typedapi.API, index, id string, doc any, options ...Option[UpdateDocRequest]) *update.Update {
	req := api.Update(index, id).Doc(doc)

	for _, option := range options {
		option(req)
	}

	return req
}

// UpdateDocAsUpsert ensures that a doc update will act as an upsert if there is no existing doc.
func UpdateDocAsUpsert() Option[UpdateDocRequest] {
	return func(req UpdateDocRequest) {
		req.DocAsUpsert(true)
	}
}

type CountRequest interface {
	RequestCommon[*count.Count]
	RequestWithIndex[*count.Count]
	RequestWithQuery[*count.Count]
}

// NewCountRequest creates a new count request with the given options.
func NewCountRequest(api *typedapi.API, options ...Option[CountRequest]) *count.Count {
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

func (v FieldValue) FieldValueCaster() *types.FieldValue {
	switch data := v.value.(type) {
	case types.FieldValue:
		return &data
	default:
		fv := types.FieldValue(data)
		return &fv
	}
}

// NewFieldValue converts any value into a FieldValue.
func NewFieldValue(value any) FieldValue {
	return FieldValue{value: value}
}

func sortByID(idField string, sortOrder models.SortOrder) types.SortOptions {
	var order *sortorder.SortOrder
	switch sortOrder {
	case models.SortOrderAsc:
		order = &sortorder.Asc
	case models.SortOrderDesc:
		fallthrough
	default:
		order = &sortorder.Desc
	}
	return types.SortOptions{
		SortOptions: map[string]types.FieldSort{
			"updated": {Order: order},
			idField:   {Order: order},
		},
	}
}

func sortByDoc() types.SortOptions {
	return types.SortOptions{
		Doc_: &types.ScoreSort{Order: &sortorder.Asc},
	}
}
