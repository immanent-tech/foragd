// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"encoding/json"
	"fmt"
	"net/url"
	"slices"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
)

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
	data, err := json.Marshal(v.value)
	if err != nil {
		return data, fmt.Errorf("failed to marshal pagination value: %w", err)
	}
	return data, nil
}

// EncodePagination will take sort values returned from a query, marshal them to
// JSON, then HTML-escape the string into a Pagination object, which is
// safe for use in API query parameters.
func EncodePagination[T ~string](sortValues []types.FieldValue) (T, error) {
	if len(sortValues) == 0 {
		return "", nil
	}
	// Marshal sort values into json.
	data, err := json.Marshal(sortValues)
	if err != nil {
		return "", fmt.Errorf("could not encode pagination values: %w", err)
	}
	// Return as HTML encoded string.
	return T(url.QueryEscape(string(data))), nil
}

// DecodePagination will take a Pagination object, HTML-unescape the
// string then unmarshal it back into sort values.
func DecodePagination[T ~string](pagination *T) ([]types.FieldValueVariant, error) {
	if pagination == nil || *pagination == "" {
		return nil, nil
	}
	// Unescape HTML encoded data.
	data, err := url.QueryUnescape(string(*pagination))
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
