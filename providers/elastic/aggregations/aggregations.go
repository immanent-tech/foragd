// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package aggregations contains objects and methods for processing Elasticsearch aggregations.
package aggregations

import (
	"errors"
	"fmt"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
)

var (
	// Aggregation Errors.
	ErrInvalidAggType = errors.New("not requested aggregation type")
)

type Aggs map[string]types.Aggregations

// ExtractAggregation extracts the named aggregation as the requested type from
// the search response.
func ExtractAggregation[T any](aggs map[string]types.Aggregate, name string) (T, error) {
	aggregation, ok := aggs[name].(T)
	if !ok {
		return aggregation, fmt.Errorf("%w: have %T, want %T", ErrInvalidAggType, aggs[name], aggregation)
	}

	return aggregation, nil
}
