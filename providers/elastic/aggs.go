// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"errors"
	"fmt"
	"slices"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
)

var (
	ErrInvalidAggType = errors.New("invalid aggregation type")
	ErrAggNotFound    = errors.New("aggregation not found")
)

type Aggs map[string]types.Aggregations

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

// ExtractAggregation recursively searches for an aggregation by name. It searches top-level Aggregations and then digs
// into bucket sub-aggregations.
func ExtractAggregation[T any](aggs map[string]types.Aggregate, name string) (T, bool, error) {
	// Search top-level.
	if agg, ok := aggs[name]; ok {
		if v, ok := agg.(T); ok {
			return v, true, nil
		} else {
			return v, false, fmt.Errorf("%w: have %T, want %T", ErrInvalidAggType, aggs[name], v)
		}
	}

	// Recurse into bucket-type aggregations that can hold sub-aggs
	for _, agg := range aggs {
		if found, ok, _ := searchInAggregate[T](agg, name); ok {
			if v, ok := found.(T); ok {
				return v, true, nil
			} else {
				return v, false, fmt.Errorf("%w: have %T, want %T", ErrInvalidAggType, aggs[name], v)
			}
		}
	}

	var v T
	return v, false, fmt.Errorf("%w: no aggregation with name %s", ErrAggNotFound, name)
}

// searchInAggregate inspects a single Aggregate value for nested aggregations.
//
//nolint:gocognit // necessarily large case statement.
func searchInAggregate[T any](agg types.Aggregate, name string) (types.Aggregate, bool, error) {
	switch extractedAgg := agg.(type) {
	case *types.StringTermsAggregate:
		switch buckets := extractedAgg.Buckets.(type) {
		case []types.StringTermsBucket:
			for bucket := range slices.Values(buckets) {
				if found, ok, err := ExtractAggregation[T](bucket.Aggregations, name); ok {
					return found, true, err
				}
			}
		case map[string]types.StringTermsBucket:
			for _, bucket := range buckets {
				if found, ok, err := ExtractAggregation[T](bucket.Aggregations, name); ok {
					return found, true, err
				}
			}
		}
	case *types.StringRareTermsAggregate:
		switch buckets := extractedAgg.Buckets.(type) {
		case []types.StringRareTermsBucket:
			for bucket := range slices.Values(buckets) {
				if found, ok, err := ExtractAggregation[T](bucket.Aggregations, name); ok {
					return found, true, err
				}
			}
		case map[string]types.StringRareTermsBucket:
			for _, bucket := range buckets {
				if found, ok, err := ExtractAggregation[T](bucket.Aggregations, name); ok {
					return found, true, err
				}
			}
		}
	case *types.DateHistogramAggregate:
		switch buckets := extractedAgg.Buckets.(type) {
		case []types.DateHistogramBucket:
			for bucket := range slices.Values(buckets) {
				if found, ok, err := ExtractAggregation[T](bucket.Aggregations, name); ok {
					return found, true, err
				}
			}
		case map[string]types.DateHistogramBucket:
			for _, bucket := range buckets {
				if found, ok, err := ExtractAggregation[T](bucket.Aggregations, name); ok {
					return found, true, err
				}
			}
		}
	}

	return nil, false, fmt.Errorf("%w: no aggregation with name %s", ErrAggNotFound, name)
}
