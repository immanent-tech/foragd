// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"slices"

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
