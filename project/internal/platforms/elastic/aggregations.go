// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"slices"

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

// Aggregation represents a named aggregation definition in a search query.
type Aggregation struct {
	Name       string
	Definition types.Aggregations
}

// TermsAggregationResults contains the results for a string terms aggregation.
type TermsAggregationResults struct {
	*types.StringTermsAggregate
}

// GetCount retrieves the document count for the bucket with the given key.
func (a *TermsAggregationResults) GetCount(key string) int64 {
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

		return value[idx].DocCount
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

// NewTermsAggregation creates a TermsAggregation aggregation for a query.
func NewTermsAggregation(name, field string) Aggregation {
	return Aggregation{
		Name: name,
		Definition: types.Aggregations{
			Terms: &types.TermsAggregation{
				Field: &field,
			},
		},
	}
}

// ExtractAggregation extracts the named aggregation as the requested type from
// the search response.
func ExtractAggregation[T any](resp *search.Response, name string) (T, error) {
	aggregation, ok := resp.Aggregations[name].(T)
	if !ok {
		return aggregation, ErrInvalidAggType
	}

	return aggregation, nil
}
