// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package aggregations contains objects and methods for processing Elasticsearch aggregations.
package aggregations

import (
	"errors"
	"fmt"
	"slices"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
)

var (
	// Aggregation Errors.
	ErrInvalidAggType    = errors.New("not requested aggregation type")
	ErrConvertFieldValue = errors.New("could not convert field value")
)

// Aggregation represents a named aggregation definition in a search query.
type Aggregation struct {
	Name       string
	Definition types.Aggregations
}

// ExtractAggregation extracts the named aggregation as the requested type from
// the search response.
func ExtractAggregation[T any](aggs map[string]types.Aggregate, name string) (T, error) {
	aggregation, ok := aggs[name].(T)
	if !ok {
		return aggregation, fmt.Errorf("%w: have %T, want %T", ErrInvalidAggType, aggs[name], aggregation)
	}

	return aggregation, nil
}

// NewTermsAggregation creates a TermsAggregation aggregation for a query.
func NewTermsAggregation(name, field string, size int) Aggregation {
	return Aggregation{
		Name: name,
		Definition: types.Aggregations{
			Terms: &types.TermsAggregation{
				Field: &field,
				Size:  &size,
			},
		},
	}
}

// AggregationResults represents the list of named aggregations from the Elasticsearch response.
type AggregationResults map[string]types.Aggregate

// TermsAggregationResults contains the results for a string terms aggregation.
type TermsAggregationResults struct {
	*types.StringTermsAggregate
}

// GetCount retrieves the document count for the bucket with the given key.
func (a *TermsAggregationResults) GetCount(key string) int {
	switch value := a.Buckets.(type) {
	case map[string]types.StringTermsBucket:
		return 0
	case []types.StringTermsBucket:
		idx := slices.IndexFunc(value, func(bucket types.StringTermsBucket) bool {
			return bucket.Key == key
		})
		if idx == -1 {
			return 0
		}

		return int(value[idx].DocCount)
	}

	return 0
}

// BucketCount retrieves the count of buckets for the terms aggregation.
func (a *TermsAggregationResults) BucketCount() int {
	switch value := a.Buckets.(type) {
	case map[string]types.StringTermsBucket:
		return len(value)
	case []types.StringTermsBucket:
		return len(value)
	}

	return 0
}

// BucketNames retrieves all the bucket names for the terms aggregation.
func (a *TermsAggregationResults) BucketNames() []string {
	switch value := a.Buckets.(type) {
	case map[string]types.StringTermsBucket:
		return nil
	case []types.StringTermsBucket:
		names := make([]string, 0, len(value))

		for _, bucket := range value {
			if category, ok := bucket.Key.(string); ok {
				names = append(names, category)
			}
		}

		return names
	}

	return nil
}

// TermsAggregationResults contains the results for a string terms aggregation.
type DiversifiedTermsAggregation struct {
	*types.DiversifiedSamplerAggregation
}
