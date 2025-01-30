// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/joshuar/go-feed-me/internal/models"
)

var (
	_ models.FeedManagementAPI = (*Client)(nil)
	_ models.FeedJobStateAPI   = (*Client)(nil)
	_ models.UserActionsAPI    = (*Client)(nil)
	_ models.UserManagementAPI = (*Client)(nil)
)

var (
	ErrNoClient      = errors.New("no client")
	ErrFieldNotFound = errors.New("field not found")
)

// ExtractSourceFromHits loops through the given hits array and extracts the `_source`
// field of each document as type `T`, returning the document sources as an array
// `[]T`. If there was an issue extracting any source, it will also return a
// non-nil error containing details.
func ExtractSourceFromHits[T any](hits []types.Hit) ([]T, error) {
	var warnings error

	sources := make([]T, 0, len(hits))

	for _, hit := range hits {
		source, err := ExtractSource[T](hit.Source_)
		if err != nil {
			warnings = errors.Join(warnings,
				fmt.Errorf("error extracting source from doc %s: %w", *hit.Id_, err))
			continue
		}

		sources = append(sources, source)
	}

	return sources, warnings
}

// ExtractSourceFromDocs loops through the given docs array and extracts the `_source`
// field of each document as type `T`, returning the document sources as an array
// `[]T`. If there was an issue extracting any source, it will also return a
// non-nil error containing details.
func ExtractSourceFromDocs[T any](docs ...any) ([]T, error) {
	var warnings error

	sources := make([]T, 0, len(docs))

	for _, doc := range docs {
		switch obj := doc.(type) {
		case types.MultiGetError:
			warnings = errors.Join(warnings, formatError(obj.Error))
		case *types.GetResult:
			source, err := ExtractSource[T](obj.Source_)
			if err != nil {
				warnings = errors.Join(warnings, err)
				continue
			}

			sources = append(sources, source)
		}
	}

	return sources, warnings
}

// ExtractSource extracts the `_source` field from a hit. A non-nil error is
// returned if the source cannot be extracted.
func ExtractSource[T any](doc json.RawMessage) (T, error) {
	var source T

	if err := json.Unmarshal(doc, &source); err != nil {
		return source, errors.Join(ErrExtractSource, err)
	}

	return source, nil
}

// ExtractSourceFromHits loops through the given hits array and extracts the `_source`
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
				fmt.Errorf("error extracting field value from doc %s: %w", *hit.Id_, err))
			continue
		}

		values[*hit.Id_] = value
	}

	return values, warnings
}

// extractFieldValue extracts the value of the given field from a hit's list of
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
