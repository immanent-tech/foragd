// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package elastic defines methods and structures for interacting with Elasticsearch.
package elastic

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
)

var (
	ErrExtractSource = errors.New("could not extract document _source")
	ErrRequestFailed = errors.New("request failed")
)

type RequestCommon[T any] interface {
	Header(key string, value string) T
}

type RequestWithQuery[T any] interface {
	Query(query types.QueryVariant) T
}

type RequestWithIDs[T any] interface {
	Ids(id ...string) T
}

type RequestWithIndex[T any] interface {
	Index(index string) T
}

type SearchRequest[T any] interface {
	RequestCommon[T]
	RequestWithQuery[T]
	Aggregations(aggregations map[string]types.Aggregations) T
}

type MSearchRequest[T any] interface {
	RequestCommon[T]
	AddSearch(header types.MultisearchHeader, body types.MultisearchBody) error
}

// WithQueryOptions option applies the given query options to the request.
func WithQueryOptions[T any, R RequestWithQuery[T]](options ...query.Option) Option[R] {
	return func(req R) {
		if query := query.Build(options...); query != nil {
			req.Query(query)
		}
	}
}

// WithRequestID option sets the appropriate request ID header to the given value in the request.
func WithRequestID[T any, V RequestCommon[T]](id string) Option[V] {
	return func(t V) {
		if id != "" {
			t.Header(ReqIDHeader, id)
		}
	}
}

// WithRequestID option sets the appropriate request ID header to the given value in the request.
func WithIndex[T any, V RequestWithIndex[T]](index string) Option[V] {
	return func(t V) {
		if index != "" {
			t.Index(index)
		}
	}
}

func WithIDs[T any, V RequestWithIDs[T]](ids ...string) Option[V] {
	return func(v V) {
		v.Ids(ids...)
	}
}

// var (
// 	_ models.FeedManagementAPI = (*Client)(nil)
// 	_ models.FeedJobStateAPI   = (*Client)(nil)
// 	_ models.UserActionsAPI    = (*Client)(nil)
// 	_ models.UserManagementAPI = (*Client)(nil)
// )

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
		return "", errors.Join(ErrPagination, fmt.Errorf("could not encode pagination values: %w", err))
	}
	// Return as HTML encoded string.
	return url.QueryEscape(string(data)), nil
}

// decodePagination will take a models.Pagination object, HTML-unescape the
// string then unmarshal it back into sort values.
func decodePagination(pagination models.Pagination) ([]types.FieldValue, error) {
	if pagination == "" {
		return nil, nil
	}
	// Unescape HTML encoded data.
	data, err := url.QueryUnescape(pagination)
	if err != nil {
		return nil, errors.Join(ErrPagination, fmt.Errorf("could not decode pagination values: %w", err))
	}
	// Unmarshal sort values.
	var sortValues []types.FieldValue
	err = json.Unmarshal([]byte(data), &sortValues)
	if err != nil {
		return nil, errors.Join(ErrPagination, fmt.Errorf("could not decode pagination values: %w", err))
	}
	// Return sort values.
	return sortValues, nil
}
