/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"

	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/providers/opensearch"
)

const (
	defaultPaginationSize = 5000
)

type Client interface {
	Search(ctx context.Context, req *opensearchapi.SearchReq) (*opensearchapi.SearchResp, error)
	Count(ctx context.Context, req *opensearchapi.CountReq) (*opensearchapi.CountResp, error)
}

// Search performs a _search request to find documents matching the given query.
func Search(
	ctx context.Context,
	client Client,
	index string,
	options ...SearchOption,
) (*SearchResponse, error) {
	req := &opensearchapi.SearchReq{
		Indices: []string{index},
		Header:  opensearch.NewRequestHeaders(ctx),
		Body:    &opensearchapi.SearchBody{},
	}

	for option := range slices.Values(options) {
		option(req)
	}

	resp, err := client.Search(ctx, req)
	if err != nil {
		for _, sub := range opensearchapi.Errors(err) {
			switch e := sub.(type) {
			case *opensearchapi.PartialSearchError:
				slogctx.Warn(ctx, "Partial failure for search request.",
					slog.Int("failed_shards", e.FailedShards),
					slog.Int("total_shards", e.TotalShards),
					slog.Int("hits", len(resp.Hits.Hits)),
				)
			default:
				return nil, opensearch.GenerateAPIError("search", resp)
			}
		}
	}

	return &SearchResponse{SearchResp: resp}, nil
}

// SearchOption is a functional option applied to the search body.
type SearchOption func(*opensearchapi.SearchReq)

// WithSize option sets the number of search results returned.
func WithSize(size int) SearchOption {
	return func(req *opensearchapi.SearchReq) {
		req.Body.Size = &size
	}
}

// WithSort option sets the sorting of the results.
func WithSort(sort *Sorting) SearchOption {
	return func(req *opensearchapi.SearchReq) {
		req.Body.Sort = new(opensearchapi.NewSortFromSortCombinations(sort.ToSortCombinations()))
	}
}

// WithFrom option sets the starting document offset. Needs to be non-negative.
func WithFrom(from int) SearchOption {
	return func(req *opensearchapi.SearchReq) {
		req.Body.From = &from
	}
}

// WithTrackTotalHits option sets whether total hits will be calculated. Disabling when not needed will increase
// performance.
func WithTrackTotalHits(value bool) SearchOption {
	return func(req *opensearchapi.SearchReq) {
		req.Body.TrackTotalHits = new(opensearchapi.NewSearchTrackHitsFromBool(value))
	}
}

// WithTrackScores option sets whether to track scores of documents. Disabling this will increase performance when
// scores are not needed.
func WithTrackScores(value bool) SearchOption {
	return func(req *opensearchapi.SearchReq) {
		req.Body.TrackScores = new(value)
	}
}

// WithQuery option sets the query used to find search results.
func WithQuery(options ...QueryOption) SearchOption {
	return func(req *opensearchapi.SearchReq) {
		if len(options) == 0 {
			return
		}
		query := &opensearchapi.CommonQueryDSLQueryContainer{}
		for option := range slices.Values(options) {
			option(query)
		}
		req.Body.Query = query
	}
}

// WithAggregation adds an aggregation to be performed on the search results.
func WithAggregation(name string, options ...AggregationOption) SearchOption {
	return func(req *opensearchapi.SearchReq) {
		if req.Body.Aggregations == nil {
			req.Body.Aggregations = make(map[string]opensearchapi.CommonAggregationsAggregationContainer)
		}
		agg := opensearchapi.CommonAggregationsAggregationContainer{}
		for option := range slices.Values(options) {
			option(&agg)
		}
		req.Body.Aggregations[name] = agg
	}
}

// WithCollapse will group results by the given field.
func WithCollapse(field string) SearchOption {
	return func(req *opensearchapi.SearchReq) {
		req.Body.Collapse = &opensearchapi.SearchFieldCollapse{
			Field: field,
		}
	}
}

type SearchAfter []opensearchapi.FieldValue

// WithSearchAfter option sets the search after parameters for a paginated search.
func WithSearchAfter(searchAfter SearchAfter) SearchOption {
	return func(req *opensearchapi.SearchReq) {
		if len(searchAfter) == 0 {
			return
		}
		req.Body.SearchAfter = searchAfter
	}
}

type SearchResponse struct {
	*opensearchapi.SearchResp
}

func (r *SearchResponse) IsError() bool {
	if resp := r.Inspect().Response; resp != nil {
		return resp.StatusCode > 399
	}
	return false
}

func (r *SearchResponse) IsPartialError() bool {
	return r.Shards.Failed > 0
}

func (r *SearchResponse) IsSuccess() bool {
	return !r.IsError() && !r.IsPartialError()
}

func (r *SearchResponse) Error() string {
	return opensearch.GenerateAPIError("search", r).Error()
}

// ExtractHits loops through the given hits array and extracts the `_source` field of each document as type `T`,
// returning the document sources as an array `[]T`. If there was an issue extracting any source, it will also return a
// non-nil error containing details. The sort value of the last hit will also be returned, which can be used for
// pagination if needed.
func (r *SearchResponse) ExtractHits[T any]() ([]T, []opensearchapi.FieldValue, error) {
	if r.IsError() {
		return nil, nil, fmt.Errorf("extract hits: %w", r)
	}

	var (
		lastSortValue []opensearchapi.FieldValue
		warnings      error
	)

	sources := make([]T, 0, len(r.Hits.Hits))

	// Loop through the hits, extracting the source into the required type.
	for hit := range slices.Values(r.Hits.Hits) {
		source, err := opensearch.ExtractSource[T](hit.Source)
		if err != nil {
			warnings = errors.Join(warnings,
				fmt.Errorf("%w (id: %s): %w", opensearch.ErrExtractSource, *hit.ID, err))
			continue
		}

		sources = append(sources, source)
	}
	// Retrieve the sort value for the last hit.
	if len(r.Hits.Hits) > 0 {
		lastSortValue = r.Hits.Hits[len(r.Hits.Hits)-1].Sort
	}

	return sources, lastSortValue, warnings
}

type Aggregate = opensearchapi.CommonAggregationsAggregate

// ExtractAggregation extracts the aggregation called name as type T, picking the matching As* method on
// [opensearchapi.CommonAggregationsAggregate] from T alone.
//
//	terms, err := resp.ExtractAggregation[opensearchapi.CommonAggregationsStringTermsAggregate]("by_status")
//
// The union carries no discriminator, so T must be the type produced by the aggregation you requested. Where several
// As* methods return the same Go type (avg/max/min/sum/... all return
// [opensearchapi.CommonAggregationsSingleMetricAggregateBase]) they share the same wire shape and required-key check,
// so the first is used.
//
// If the response was requested with typed_keys=true, the map key is prefixed ("sterms#by_status"); Get matches on
// either form.
func (r *SearchResponse) ExtractAggregation[T any](name string) (T, error) {
	var zero T

	agg, ok := lookup(r.Aggregations, name) // local copy: As* have pointer receivers, map values aren't addressable
	if !ok {
		return zero, fmt.Errorf("%w: not found: %q", opensearch.ErrExtractAggregation, name)
	}
	if agg.IsZero() {
		return zero, fmt.Errorf("%w: %q", opensearch.ErrAggregationEmpty, name)
	}

	var (
		v   any
		err error
	)
	switch any(zero).(type) {
	// ---- metrics ----
	case opensearchapi.CommonAggregationsSingleMetricAggregateBase: // avg, max, min, sum, value_count, simple_value, median_absolute_deviation, weighted_avg
		v, err = agg.AsAvg()
	case opensearchapi.CommonAggregationsCardinalityAggregate:
		v, err = agg.AsCardinality()
	case opensearchapi.CommonAggregationsCumulativeCardinalityAggregate: // AsSimpleLongValue
		v, err = agg.AsSimpleLongValue()
	case opensearchapi.CommonAggregationsStatsAggregateBase: // stats, stats_bucket
		v, err = agg.AsStats()
	case opensearchapi.CommonAggregationsExtendedStatsAggregateBase: // extended_stats, extended_stats_bucket
		v, err = agg.AsExtendedStats()
	case opensearchapi.CommonAggregationsPercentilesAggregateBase: // percentiles/ranks (hdr, tdigest), percentiles_bucket
		v, err = agg.AsTDigestPercentiles()
	case opensearchapi.CommonAggregationsBoxPlotAggregate:
		v, err = agg.AsBoxPlot()
	case opensearchapi.CommonAggregationsBucketMetricValueAggregate:
		v, err = agg.AsBucketMetricValue()
	case opensearchapi.CommonAggregationsDerivativeAggregate:
		v, err = agg.AsDerivative()
	case opensearchapi.CommonAggregationsGeoBoundsAggregate:
		v, err = agg.AsGeoBounds()
	case opensearchapi.CommonAggregationsGeoCentroidAggregate:
		v, err = agg.AsGeoCentroid()
	case opensearchapi.CommonAggregationsMatrixStatsAggregate:
		v, err = agg.AsMatrixStats()
	case opensearchapi.CommonAggregationsRateAggregate:
		v, err = agg.AsRate()
	case opensearchapi.CommonAggregationsScriptedMetricAggregate:
		v, err = agg.AsScriptedMetric()
	case opensearchapi.CommonAggregationsTTestAggregate:
		v, err = agg.AsTTest()
	case opensearchapi.CommonAggregationsTopHitsAggregate:
		v, err = agg.AsTopHits()

	// ---- single-bucket ----
	case opensearchapi.CommonAggregationsSingleBucketAggregateBase: // filter, global, missing, nested, reverse_nested, parent, children, sampler
		v, err = agg.AsFilter()

	// ---- multi-bucket ----
	case opensearchapi.CommonAggregationsAdjacencyMatrixAggregate:
		v, err = agg.AsAdjacencyMatrix()
	case opensearchapi.CommonAggregationsAutoDateHistogramAggregate:
		v, err = agg.AsAutoDateHistogram()
	case opensearchapi.CommonAggregationsCompositeAggregate:
		v, err = agg.AsComposite()
	case opensearchapi.CommonAggregationsDateHistogramAggregate:
		v, err = agg.AsDateHistogram()
	case opensearchapi.CommonAggregationsFiltersAggregate:
		v, err = agg.AsFilters()
	case opensearchapi.CommonAggregationsGeoHashGridAggregate:
		v, err = agg.AsGeohashGrid()
	case opensearchapi.CommonAggregationsGeoTileGridAggregate:
		v, err = agg.AsGeotileGrid()
	case opensearchapi.CommonAggregationsHistogramAggregate:
		v, err = agg.AsHistogram()
	case opensearchapi.CommonAggregationsIPRangeAggregate:
		v, err = agg.AsIPRange()
	case opensearchapi.CommonAggregationsMultiBucketAggregateBaseRangeBucket: // range, date_range, geo_distance
		v, err = agg.AsRange()
	case opensearchapi.CommonAggregationsMultiTermsAggregate:
		v, err = agg.AsMultiTerms()
	case opensearchapi.CommonAggregationsVariableWidthHistogramAggregate:
		v, err = agg.AsVariableWidthHistogram()

	// ---- terms family ----
	case opensearchapi.CommonAggregationsStringTermsAggregate:
		v, err = agg.AsSTerms()
	case opensearchapi.CommonAggregationsLongTermsAggregate:
		v, err = agg.AsLTerms()
	case opensearchapi.CommonAggregationsDoubleTermsAggregate:
		v, err = agg.AsDTerms()
	case opensearchapi.CommonAggregationsUnsignedLongTermsAggregate:
		v, err = agg.AsULTerms()
	case opensearchapi.CommonAggregationsUnmappedTermsAggregate:
		v, err = agg.AsUMTerms()
	case opensearchapi.CommonAggregationsStringRareTermsAggregate:
		v, err = agg.AsSRareTerms()
	case opensearchapi.CommonAggregationsLongRareTermsAggregate:
		v, err = agg.AsLRareTerms()
	case opensearchapi.CommonAggregationsUnmappedRareTermsAggregate:
		v, err = agg.AsUMRareTerms()
	case opensearchapi.CommonAggregationsSignificantStringTermsAggregate:
		v, err = agg.AsSigSTerms()
	case opensearchapi.CommonAggregationsSignificantLongTermsAggregate:
		v, err = agg.AsSigLTerms()
	case opensearchapi.CommonAggregationsUnmappedSignificantTermsAggregate:
		v, err = agg.AsUMSigTerms()

	default:
		return zero, fmt.Errorf("%w: unsupported type: %T", opensearch.ErrExtractAggregation, zero)
	}
	if err != nil {
		return zero, fmt.Errorf("aggregation %q as %T: %w", name, zero, err)
	}
	return v.(T), nil
}

// MustGetAggregation is Get that panics on error. Handy in tests.
func (r *SearchResponse) MustGetAggregation[T any](name string) T {
	v, err := r.ExtractAggregation[T](name)
	if err != nil {
		panic(err)
	}
	return v
}

// lookup finds name directly, or as a typed_keys entry ("<type>#<name>").
func lookup(m map[string]Aggregate, name string) (Aggregate, bool) {
	if a, ok := m[name]; ok {
		return a, true
	}
	suffix := "#" + name
	for k, a := range m {
		if strings.HasSuffix(k, suffix) {
			return a, true
		}
	}
	return Aggregate{}, false
}

// SearchAll performs a _search request to find documents matching the given query.
func SearchAll(
	ctx context.Context,
	client Client,
	index string,
	paginationSize int,
	queries []QueryOption,
	options ...SearchOption,
) (*SearchAllResponse, error) {
	if paginationSize <= 0 {
		paginationSize = defaultPaginationSize
	}

	var searchAfter []opensearchapi.FieldValue

	resp := SearchAllResponse{
		Responses: make([]*opensearchapi.SearchResp, 0),
	}

	for {
		body := &opensearchapi.SearchBody{}

		loopReq := &opensearchapi.SearchReq{
			Indices: []string{index},
			Header:  opensearch.NewRequestHeaders(ctx),
			Body:    body,
		}
		WithQuery(queries...)(loopReq)
		WithSearchAfter(searchAfter)(loopReq)
		WithSearchParameters(WithSortParams("_doc"))(loopReq)
		WithTrackTotalHits(false)(loopReq)
		WithTrackScores(false)(loopReq)
		WithSize(paginationSize)(loopReq)

		// Set additional options.
		for option := range slices.Values(options) {
			option(loopReq)
		}

		loopResp, err := client.Search(ctx, loopReq)
		if err != nil {
			for _, sub := range opensearchapi.Errors(err) {
				switch e := sub.(type) {
				case *opensearchapi.PartialSearchError:
					slogctx.Warn(ctx, "Partial failure for search request.",
						slog.Int("failed_shards", e.FailedShards),
						slog.Int("total_shards", e.TotalShards),
						slog.Int("hits", len(loopResp.Hits.Hits)),
					)
				default:
					resp.errors = append(resp.errors, opensearch.GenerateAPIError("search", loopResp))
				}
			}
		}

		// Add response to responses slice.
		resp.Responses = append(resp.Responses, loopResp)

		// Stop looping if we've hit the end of the results.
		if len(loopResp.Hits.Hits) < paginationSize {
			break
		}

		// Update search after.
		searchAfter = loopResp.Hits.Hits[len(loopResp.Hits.Hits)-1].Sort
	}
	return &resp, nil
}

type SearchAllResponse struct {
	Responses []*opensearchapi.SearchResp
	errors    []error
}

func (r *SearchAllResponse) IsError() bool {
	if len(r.errors) > 0 {
		return true
	}
	for resp := range slices.Values(r.Responses) {
		if resp := resp.Inspect().Response; resp != nil {
			if resp.StatusCode > 399 {
				return true
			}
		}
	}
	return false
}

func (r *SearchAllResponse) IsPartialError() bool {
	if len(r.errors) > 0 {
		return true
	}
	for resp := range slices.Values(r.Responses) {
		return resp.Shards.Failed > 0
	}
	return false
}

func (r *SearchAllResponse) IsSuccess() bool {
	if len(r.errors) > 0 {
		return false
	}
	return !r.IsError() && !r.IsPartialError()
}

func (r *SearchAllResponse) Error() string {
	var respErrs []error
	for resp := range slices.Values(r.Responses) {
		respErrs = append(respErrs, opensearch.GenerateAPIError("search", resp))
	}
	r.errors = append(r.errors, respErrs...)
	return errors.Join(r.errors...).Error()
}

func (r *SearchAllResponse) ExtractHits[T any]() ([]T, error) {
	var warnings error

	allHits := make([]T, 0)

	for resp := range slices.Values(r.Responses) {
		hits := make([]T, 0, len(resp.Hits.Hits))
		// Loop through the hits of this response, extracting the source into the required type.
		for hit := range slices.Values(resp.Hits.Hits) {
			source, err := opensearch.ExtractSource[T](hit.Source)
			if err != nil {
				warnings = errors.Join(warnings,
					fmt.Errorf("%w (id: %s): %w", opensearch.ErrExtractSource, *hit.ID, err))
				continue
			}

			hits = append(hits, source)
		}
		// Append hits from this response to all hits.
		allHits = append(allHits, hits...)
	}

	return allHits, warnings
}

func (r *SearchAllResponse) TotalHits() int64 {
	var hits int64
	for resp := range slices.Values(r.Responses) {
		if resp == nil {
			continue
		}
		if resp.Hits.Total == nil {
			continue
		}
		h, err := resp.Hits.Total.Int64()
		if err != nil {
			continue
		}
		hits += h
	}
	return hits
}

func Count(ctx context.Context, client Client, index string, options ...CountOption) (int64, error) {
	// Apply specified options.
	body := &opensearchapi.CountBody{}
	for option := range slices.Values(options) {
		option(body)
	}

	// Generate request.
	req := &opensearchapi.CountReq{
		Indices: []string{index},
		Header:  opensearch.NewRequestHeaders(ctx),
		Body:    body,
	}

	// Perform request.
	resp, err := client.Count(ctx, req)
	if err != nil {
		for _, sub := range opensearchapi.Errors(err) {
			switch e := sub.(type) {
			case *opensearchapi.PartialSearchError:
				slogctx.Warn(ctx, "Partial failure for search request.",
					slog.Int("failed_shards", e.FailedShards),
					slog.Int("total_shards", e.TotalShards),
				)
			default:
				return 0, opensearch.GenerateAPIError("search", resp)
			}
		}
	}

	return resp.Count, nil
}

// CountOption is a functional option that applies to a count request.
type CountOption func(*opensearchapi.CountBody)

// WithCountQuery option sets the query used for filtering the count.
func WithCountQuery(options ...QueryOption) CountOption {
	return func(req *opensearchapi.CountBody) {
		query := &opensearchapi.CommonQueryDSLQueryContainer{}
		for option := range slices.Values(options) {
			option(query)
		}
		req.Query = query
	}
}

// EncodePagination will take sort values returned from a query, marshal them to
// JSON, then HTML-escape the string into a Pagination object, which is
// safe for use in API query parameters.
func EncodePagination[T ~string](sortValues SearchAfter) (T, error) {
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
func DecodePagination[T ~string](pagination *T) (SearchAfter, error) {
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
	casted := make(SearchAfter, 0, len(values))
	for v := range slices.Values(values) {
		switch r := v.(type) {
		case string:
			casted = append(casted, opensearchapi.NewFieldValueFromString(r))
		case float64:
			casted = append(casted, opensearchapi.NewFieldValueFromFloat64(r))
		case bool:
			casted = append(casted, opensearchapi.NewFieldValueFromBool(r))
		default:
			casted = nil
		}
	}

	// Return sort values.
	return casted, nil
}
