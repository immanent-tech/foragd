// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"slices"

	"github.com/elastic/go-elasticsearch/v8/typedapi"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/count"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/sortorder"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
)

// SearchOption is a functional option to apply to a search request.
type SearchOption Option[*search.Search]

// CountOption is a functional option to apply to a count request.
type CountOption Option[*count.Count]

type SearchRequest struct {
	*search.Search
	sortKeyOrder []string
}

// SearchResult represents a search request result.
type SearchResult struct {
	*search.Response
}

// WithSearchIndex sets the index (or index pattern) to search over.
func WithSearchIndex(index string) SearchOption {
	return func(s *search.Search) {
		s.Index(index)
	}
}

// WithAggregations adds the given aggregation definitions to the search.
func WithAggregations(definitions ...Aggregation) SearchOption {
	return func(search *search.Search) {
		aggregations := make(map[string]types.Aggregations)

		for _, definition := range definitions {
			aggregations[definition.Name] = definition.Definition
		}

		search.Aggregations(aggregations)
	}
}

// WithSearchQueryOptions adds the given query options (conditions) to the search.
func WithSearchQueryOptions(options ...query.Option) SearchOption {
	return func(search *search.Search) {
		if query := query.Build(options...); query != nil {
			search.Query(query)
		}
	}
}

// WithSortOptions adds the given sorting options to the search.
func WithSortOptions(options ...types.SortCombinationsVariant) SearchOption {
	return func(search *search.Search) {
		search.Sort(options...)
	}
}

// WithFields ensures the search will return the given fields in the response.
func WithFields(fields ...string) SearchOption {
	return func(search *search.Search) {
		fieldsReturned := make([]types.FieldAndFormatVariant, len(fields))
		for i, name := range fields {
			fieldsReturned[i] = &types.FieldAndFormat{Field: name}
		}

		search.Fields(fieldsReturned...)
	}
}

// WithSearchSize defines the number of results returned.
func WithSearchSize(size int) SearchOption {
	return func(search *search.Search) {
		search.Size(size)
	}
}

// WithSearchAfter sets the sort value to fetch the next set of results. It can
// accept either a []types.FieldValue or a []byte (html-encoded
// []types.FieldValue).
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/paginate-search-results.html#search-after
func WithSearchAfter(value any) SearchOption {
	return func(search *search.Search) {
		if value == nil {
			return
		}

		if values, ok := value.([]types.FieldValue); ok {
			fieldValues := make([]types.FieldValueVariant, 0, len(values))
			for value := range slices.Values(values) {
				fieldValues = append(fieldValues, NewFieldValue(value))
			}
			search.SearchAfter(fieldValues...)
		} else {
			search.SearchAfter(NewFieldValue(value))
		}
	}
}

// NewSearchRequest creates a new search request with the given options.
func NewSearchRequest(api *typedapi.API, options ...SearchOption) *search.Search {
	req := api.Search()

	for _, option := range options {
		option(req)
	}

	return req
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
