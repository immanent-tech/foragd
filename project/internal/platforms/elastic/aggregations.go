// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"slices"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

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
