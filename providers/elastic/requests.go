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
	"github.com/joshuar/go-feed-me/providers/elastic/query"
)

// CountOption is a functional option to apply to a count request.
type CountOption Option[*count.Count]

type SearchRequest[T any] interface {
	RequestCommon[T]
	RequestWithIndex[T]
	RequestWithQuery[T]
	RequestWithAggregations[T]
	RequestWithSize[T]
	RequestWithSearchAfter[T]
	RequestWithSort[T]
}

type SearchAPIRequest SearchRequest[*search.Search]

// NewSearchRequest creates a new search request with the given options.
func NewSearchRequest(api *typedapi.API, options ...Option[SearchAPIRequest]) *search.Search {
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
		searchBody.Sort = search.GenerateSortCombination()

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

// WithCountIndex sets the index (or index pattern) to search over.
func WithCountIndex(index string) CountOption {
	return func(s *count.Count) {
		s.Index(index)
	}
}

// WithCountQueryOptions adds the given query options (conditions) to the count.
func WithCountQueryOptions(options ...query.Option) CountOption {
	return func(count *count.Count) {
		queryOptions := &types.Query{}

		for _, option := range options {
			option(queryOptions)
		}

		count.Query(queryOptions)
	}
}

// NewCountRequest creates a new count request with the given options.
func NewCountRequest(api *typedapi.API, options ...CountOption) *count.Count {
	req := api.Count()

	for _, option := range options {
		option(req)
	}

	return req
}

// FieldSort represents the sorting order for a field.
type FieldSort struct {
	field string
	order models.SortOrder
}

func (s FieldSort) SortCombinationsCaster() *types.SortCombinations {
	opts := types.NewSortOptions()
	switch s.order {
	case models.SortOrderAsc:
		opts.SortOptions[s.field] = types.FieldSort{Order: &sortorder.Asc}
	case models.SortOrderDesc:
		opts.SortOptions[s.field] = types.FieldSort{Order: &sortorder.Desc}
	}
	sort := types.SortCombinations(opts)
	return &sort
}

// NewFieldSort creates a new FieldSort for the given field with the given sort order.
func NewFieldSort(field string, order models.SortOrder) FieldSort {
	return FieldSort{field: field, order: order}
}

// SortTimestampDesc returns a sort parameter for a search that will sort
// results by the @timestamp field in descending order.
func SortTimestampDesc() FieldSort {
	return NewFieldSort("@timestamp", models.SortOrderDesc)
}

// SortByDocID will sort search results by `_doc`, effectively unordered but
// most efficient sorting. Useful when paginating through *all* docs.
func SortByDocID(field string) FieldSort {
	return NewFieldSort(field, models.SortOrderDesc)
}

type ScoreSort struct{}

func (s ScoreSort) SortCombinationsCaster() *types.SortCombinations {
	opts := types.NewSortOptions()
	opts.Score_ = types.NewScoreSort()
	sort := types.SortCombinations(opts)
	return &sort
}

func SortByScore() ScoreSort {
	return ScoreSort{}
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
