// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"encoding/json"
	"log/slog"

	"github.com/elastic/go-elasticsearch/v8/typedapi"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/count"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/sortorder"
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
func WithSearchQueryOptions(options ...QueryOption) SearchOption {
	return func(search *search.Search) {
		if query := BuildQuery(options...); query != nil {
			search.Query(query)
		}
	}
}

// WithSortOptions adds the given sorting options to the search.
func WithSortOptions(options map[string]types.FieldSort) SearchOption {
	return func(search *search.Search) {
		search.Sort(options)
	}
}

// WithFields ensures the search will return the given fields in the response.
func WithFields(fields ...string) SearchOption {
	return func(search *search.Search) {
		fieldsReturned := make([]types.FieldAndFormat, len(fields))
		for i, name := range fields {
			fieldsReturned[i] = types.FieldAndFormat{Field: name}
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
		if value != nil {
			switch sort := value.(type) {
			case []types.FieldValue:
				search.SearchAfter(sort...)
			case []byte:
				if string(sort) != "" {
					var fv []types.FieldValue
					if err := json.Unmarshal(sort, &fv); err != nil {
						slog.Warn("Could not unmarshal pagination data.", slog.Any("error", err))
					} else {
						search.SearchAfter(fv...)
					}
				}
			}
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
func WithCountQueryOptions(options ...QueryOption) CountOption {
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

// SortTimestampDesc returns a sort parameter for a search that will sort
// results by the @timestamp field in descending order.
func SortTimestampDesc() map[string]types.FieldSort {
	return map[string]types.FieldSort{"@timestamp": {Order: &sortorder.Desc}}
}

// SortByDocID will sort search results by `_doc`, effectively unordered but
// most efficient sorting. Useful when paginating through *all* docs.
func SortByDocID(idField string) map[string]types.FieldSort {
	return map[string]types.FieldSort{
		"_doc":  {},
		idField: {Order: &sortorder.Desc},
	}
}
