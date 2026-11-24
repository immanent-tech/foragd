/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package search

import (
	"encoding/json"
	"slices"

	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"
)

type AggregationOption func(*opensearchapi.CommonAggregationsAggregationContainer)

// WithChildAggregation option defines a child [AggregationOption]. Can be nested.
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

func WithTermsAggregationExclude(values ...string) TermsAggregationOption {
	return func(a *opensearchapi.CommonAggregationsTermsAggregationFields) {
		a.Exclude = new(opensearchapi.NewCommonAggregationsTermsExcludeFromArray(values))
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

func WithTopHitsAggregationSort(sort *Sorting) TopHitsAggregationOption {
	return func(a *opensearchapi.CommonAggregationsTopHitsAggregation) {
		v := opensearchapi.NewSortFromSortCombinations(sort.ToSortCombinations())
		a.Sort = &v
	}
}

// FiltersAggregation creates multiple buckets, each associated with a named or anonymous filter query.
//
// https://docs.opensearch.org/latest/aggregations/bucket/filters/
func FiltersAggregation(options ...FiltersAggregationOption) AggregationOption {
	return func(a *opensearchapi.CommonAggregationsAggregationContainer) {
		fields := &opensearchapi.CommonAggregationsFiltersAggregationFields{}
		for option := range slices.Values(options) {
			option(fields)
		}
		a.Filters = fields
	}
}

// FiltersAggregationOption is a functional option applied to a [FiltersAggregation].
type FiltersAggregationOption func(*opensearchapi.CommonAggregationsFiltersAggregationFields)

// WithFiltersAggregationFilters option defines named filters that will become the buckets of the [FiltersAggregation].
func WithFiltersAggregationFilters(filters map[string]QueryOption) FiltersAggregationOption {
	return func(a *opensearchapi.CommonAggregationsFiltersAggregationFields) {
		queries := make(map[string]json.RawMessage)
		for key, query := range filters {
			f := opensearchapi.CommonQueryDSLQueryContainer{}
			query(&f)
			data, err := json.Marshal(&f)
			if err != nil {
				continue
			}
			queries[key] = data
		}
		a.Filters = new(opensearchapi.NewCommonAggregationsBucketsQueryContainerFromMap(queries))
	}
}

// RareTermsAggregation is a bucket aggregation that identifies infrequent terms in a dataset.
func RareTermsAggregation(options ...RareTermsAggregationOption) AggregationOption {
	return func(a *opensearchapi.CommonAggregationsAggregationContainer) {
		terms := &opensearchapi.CommonAggregationsRareTermsAggregationFields{}
		for option := range slices.Values(options) {
			option(terms)
		}
		a.RareTerms = terms
	}
}

// RareTermsAggregationOption is a functional option applied to a [RareTermsAggregation].
type RareTermsAggregationOption func(*opensearchapi.CommonAggregationsRareTermsAggregationFields)

func WithRareTermsAggregationField(field string) RareTermsAggregationOption {
	return func(a *opensearchapi.CommonAggregationsRareTermsAggregationFields) {
		a.Field = &field
	}
}

func WithRareTermsAggregationMaxDocCount(size int64) RareTermsAggregationOption {
	return func(a *opensearchapi.CommonAggregationsRareTermsAggregationFields) {
		a.MaxDocCount = &size
	}
}

func WithRareTermsAggregationExclude(values ...string) RareTermsAggregationOption {
	return func(a *opensearchapi.CommonAggregationsRareTermsAggregationFields) {
		a.Exclude = new(opensearchapi.NewCommonAggregationsTermsExcludeFromArray(values))
	}
}

func DateHistogramAggregation(field string, options ...DateHistogramAggregationOption) AggregationOption {
	return func(a *opensearchapi.CommonAggregationsAggregationContainer) {
		histo := &opensearchapi.CommonAggregationsDateHistogramAggregationFields{
			Field: &field,
		}
		for option := range slices.Values(options) {
			option(histo)
		}
		a.DateHistogram = histo
	}
}

type DateHistogramAggregationOption func(*opensearchapi.CommonAggregationsDateHistogramAggregationFields)

func WithDateHistogramAggregationCalendarInterval(interval string) DateHistogramAggregationOption {
	return func(a *opensearchapi.CommonAggregationsDateHistogramAggregationFields) {
		a.CalendarInterval = &interval
	}
}

func WithDateHistogramAggregationFormat(format string) DateHistogramAggregationOption {
	return func(a *opensearchapi.CommonAggregationsDateHistogramAggregationFields) {
		a.Format = &format
	}
}

func AverageBucketAggregation(options ...PipelineAggregationOption) AggregationOption {
	return func(a *opensearchapi.CommonAggregationsAggregationContainer) {
		avgBucket := &opensearchapi.CommonAggregationsPipelineAggregationBase{}
		for option := range slices.Values(options) {
			option(avgBucket)
		}
		a.AvgBucket = avgBucket
	}
}

type PipelineAggregationOption func(*opensearchapi.CommonAggregationsPipelineAggregationBase)

func WithPipelineAggregationBucketsPath(path string) PipelineAggregationOption {
	return func(a *opensearchapi.CommonAggregationsPipelineAggregationBase) {
		a.BucketsPath = new(opensearchapi.NewCommonAggregationsBucketsPathFromString(path))
	}
}

// DiversifiedSamplerAggregation is a filtering aggregation that limits subaggregation processing to a sample of
// top-scoring documents while ensuring the sample contains diverse content.
//
// https://docs.opensearch.org/latest/aggregations/bucket/diversified-sampler/
func DiversifiedSamplerAggregation(options ...DiversifiedSamplerAggregationOption) AggregationOption {
	return func(a *opensearchapi.CommonAggregationsAggregationContainer) {
		sampler := &opensearchapi.CommonAggregationsDiversifiedSamplerAggregationFields{}
		for option := range slices.Values(options) {
			option(sampler)
		}
		a.DiversifiedSampler = sampler
	}
}

// DiversifiedSamplerAggregationOption is a functional option applied to a [DiversifiedSamplerAggregation].
type DiversifiedSamplerAggregationOption func(*opensearchapi.CommonAggregationsDiversifiedSamplerAggregationFields)

// WithDiversifiedSamplerAggregationField option sets the field used for deduplication. Must produce a single value per
// document.
func WithDiversifiedSamplerAggregationField(field string) DiversifiedSamplerAggregationOption {
	return func(a *opensearchapi.CommonAggregationsDiversifiedSamplerAggregationFields) {
		a.Field = &field
	}
}

// WithDiversifiedSamplerAggregationShardSize options is the maximum number of top-scoring documents collected on each
// shard.
func WithDiversifiedSamplerAggregationShardSize(size int) DiversifiedSamplerAggregationOption {
	return func(a *opensearchapi.CommonAggregationsDiversifiedSamplerAggregationFields) {
		a.ShardSize = &size
	}
}

// WithDiversifiedSamplerAggregationMaxDocsPerValue option is the cap on how many documents sharing the same
// deduplication value can enter the sample on each shard. Default is 1, so the sample contains at most one document for
// each value.
func WithDiversifiedSamplerAggregationMaxDocsPerValue(max int) DiversifiedSamplerAggregationOption {
	return func(a *opensearchapi.CommonAggregationsDiversifiedSamplerAggregationFields) {
		a.MaxDocsPerValue = &max
	}
}
