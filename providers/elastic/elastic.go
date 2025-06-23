// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package elastic defines methods and structures for interacting with Elasticsearch.
package elastic

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/joshuar/go-feed-me/providers/elastic/aggregations"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
)

var (
	ErrExtractSource = errors.New("could not extract document _source")
	ErrRequestFailed = errors.New("request failed")
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
	SearchAfter(values ...types.FieldValueVariant) T
}

// WithSearchAfter sets the sort value to fetch the next set of results. It can
// accept either a []types.FieldValue or a []byte (html-encoded
// []types.FieldValue).
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/paginate-search-results.html#search-after
func WithSearchAfter[T any, V RequestWithSearchAfter[T]](value any) Option[V] {
	return func(req V) {
		if value == nil {
			return
		}

		if values, ok := value.([]types.FieldValue); ok {
			fieldValues := make([]types.FieldValueVariant, 0, len(values))
			for value := range slices.Values(values) {
				fieldValues = append(fieldValues, NewFieldValue(value))
			}
			req.SearchAfter(fieldValues...)
		} else {
			req.SearchAfter(NewFieldValue(value))
		}
	}
}

type RequestWithSort[T any] interface {
	Sort(sort ...types.SortCombinationsVariant) T
}

// WithSortOptions adds the given sorting options to the search.
func WithSortOptions[T any, V RequestWithSort[T]](options ...types.SortCombinationsVariant) Option[V] {
	return func(req V) {
		req.Sort(options...)
	}
}

// ExtractSourceFromHits loops through the given hits array and extracts the `_source`
// field of each document as type `T`, returning the document sources as an array
// `[]T`. If there was an issue extracting any source, it will also return a
// non-nil error containing details. The sort value of the last hit will also be
// returned, which can be used for pagination if needed.
func ExtractSourceFromHits[T any](hits []types.Hit) ([]T, []types.FieldValue, error) {
	var (
		lastSortValue []types.FieldValue
		warnings      error
	)

	sources := make([]T, 0, len(hits))

	// Loop through the hits, extracting the source into the required type.
	for _, hit := range hits {
		source, err := ExtractSource[T](hit.Source_)
		if err != nil {
			warnings = errors.Join(warnings,
				fmt.Errorf("%w (id: %s): %w", ErrExtractSource, *hit.Id_, err))
			continue
		}

		sources = append(sources, source)
	}
	// Retrieve the sort value for the last hit.
	if len(hits) > 0 {
		lastSortValue = hits[len(hits)-1].Sort
	}

	return sources, lastSortValue, warnings
}

// ExtractSourceFromDocs loops through the given docs array and extracts the `_source`
// field of each document as type `T`, returning the document sources as an array
// `[]T`. If there was an issue extracting any source, it will also return a
// non-nil error containing details.
func ExtractSourceFromDocs[T any](docs []types.MgetResponseItem) ([]T, error) {
	var warnings error

	sources := make([]T, 0, len(docs))

	for doc := range slices.Values(docs) {
		switch obj := doc.(type) {
		case types.MultiGetError:
			warnings = errors.Join(warnings, formatError(obj.Error))
		case *types.GetResult:
			if !obj.Found {
				continue
			}
			source, err := ExtractSource[T](obj.Source_)
			if err != nil {
				warnings = errors.Join(warnings, err)
				continue
			}
			sources = append(sources, source)
		default:
			warnings = errors.Join(warnings, errors.New("unknown doc type"))
		}
	}

	return sources, warnings
}

// ExtractSource extracts the `_source` field from a hit. A non-nil error is
// returned if the source cannot be extracted.
func ExtractSource[T any](doc json.RawMessage) (T, error) {
	var source T

	if err := json.Unmarshal(doc, &source); err != nil {
		return source, fmt.Errorf("%w: %w", ErrExtractSource, err)
	}

	return source, nil
}

// ExtractFieldFromHits loops through the given hits array and extracts the `_source`
// field of each document as type `T`, returning the document sources as an array
// `[]T`. If there was an issue extracting any source, it will also return a
// non-nil error containing details.
func ExtractFieldFromHits[T any](field string, hits []types.Hit) (map[string]T, error) {
	var warnings error

	values := make(map[string]T)

	for _, hit := range hits {
		value, err := ExtractFieldValue[T](field, hit.Fields)
		if err != nil {
			warnings = errors.Join(warnings,
				fmt.Errorf("%w (id: %s): %w", ErrExtractSource, *hit.Id_, err))
			continue
		}
		values[*hit.Id_] = value
	}

	return values, warnings
}

// ExtractFieldValue extracts the value of the given field from a hit's list of
// returned fields. If the field is not found or the value cannot be extracted,
// a non-nil error is returned.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/search-fields.html#search-fields-param
func ExtractFieldValue[T any](field string, fields map[string]json.RawMessage) (T, error) {
	var fieldValue []T

	value, found := fields[field]
	if !found {
		return fieldValue[0], ErrFieldNotFound
	}

	err := json.Unmarshal(value, &fieldValue)
	if err != nil {
		return fieldValue[0], errors.Join(ErrFieldNotFound, err)
	}

	return fieldValue[0], nil
}

// formatError formats an error cause from Elasticsearch into an error value.
func formatError(err types.ErrorCause) error {
	return fmt.Errorf("%s: %s", err.Type, *err.Reason)
}
