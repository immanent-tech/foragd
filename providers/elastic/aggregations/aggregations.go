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
	ErrInvalidAggType = errors.New("invalid aggregation type")
)

type Aggs map[string]types.Aggregations

// ExtractAggregation extracts the named aggregation as the requested type from the search response. If the aggregation
// cannot be extracted to the type, a non-nil error is returned that will contain details.
func ExtractAggregation[T any](aggs map[string]types.Aggregate, name string) (T, error) {
	aggregation, ok := aggs[name].(T)
	if !ok {
		return aggregation, fmt.Errorf("%w: have %T, want %T", ErrInvalidAggType, aggs[name], aggregation)
	}

	return aggregation, nil
}

// BucketTypes is a type constraint for all aggregation bucket types.
type BucketTypes interface {
	types.StringTermsBucket | types.StringRareTermsBucket
}

// ExtractBuckets extracts the bucket object from the aggregation as the given type. If the aggregation cannot be
// extracted to the type, a non-nil error is returned that will contain details.
func ExtractBuckets[T BucketTypes](container any) ([]T, error) {
	buckets, ok := container.([]T)
	if !ok {
		return buckets, fmt.Errorf("%w: have %T, want %T", ErrInvalidAggType, container, buckets)
	}
	return buckets, nil
}
