/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package search

import (
	"slices"

	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"
)

type AggregationOption func(*opensearchapi.CommonAggregationsAggregationContainer)

func WithChildAggregation(name string, options ...AggregationOption) AggregationOption {
	return func(a *opensearchapi.CommonAggregationsAggregationContainer) {
		if a.Aggregations == nil {
			a.Aggregations = make(map[string]opensearchapi.CommonAggregationsAggregationContainer)
		}
		agg := opensearchapi.CommonAggregationsAggregationContainer{}
		for option := range slices.Values(options) {
			option(&agg)
		}
		a.Aggregations[name] = agg
	}
}

func TermsAggregation(options ...TermsAggregationOption) AggregationOption {
	return func(a *opensearchapi.CommonAggregationsAggregationContainer) {
		terms := &opensearchapi.CommonAggregationsTermsAggregationFields{}
		for option := range slices.Values(options) {
			option(terms)
		}
		a.Terms = terms
	}
}

type TermsAggregationOption func(*opensearchapi.CommonAggregationsTermsAggregationFields)

func WithTermsAggregationField(field string) TermsAggregationOption {
	return func(a *opensearchapi.CommonAggregationsTermsAggregationFields) {
		a.Field = &field
	}
}

func WithTermsAggregationSize(size int) TermsAggregationOption {
	return func(a *opensearchapi.CommonAggregationsTermsAggregationFields) {
		a.Size = &size
	}
}

func TopHitsAggregation(options ...TopHitsAggregationOption) AggregationOption {
	return func(a *opensearchapi.CommonAggregationsAggregationContainer) {
		topHits := &opensearchapi.CommonAggregationsTopHitsAggregation{}
		for option := range slices.Values(options) {
			option(topHits)
		}
		a.TopHits = topHits
	}
}

type TopHitsAggregationOption func(*opensearchapi.CommonAggregationsTopHitsAggregation)

func WithTopHitsAggregationSize(size int) TopHitsAggregationOption {
	return func(a *opensearchapi.CommonAggregationsTopHitsAggregation) {
		a.Size = &size
	}
}

func WithTopHitsAggregationSort(sort string) TopHitsAggregationOption {
	return func(a *opensearchapi.CommonAggregationsTopHitsAggregation) {
		v := opensearchapi.NewSortFromSortCombinations(opensearchapi.NewSortCombinationsFromString(sort))
		a.Sort = &v
	}
}
