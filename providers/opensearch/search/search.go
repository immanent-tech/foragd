/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package search

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/goforj/godump"
	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/opensearch"
)

const (
	defaultPaginationSize = 5000
)

// Search performs a _search request to find documents matching the given query.
func Search(
	ctx context.Context,
	index string,
	options ...SearchOption,
) (*SearchResponse, error) {
	// Connect to elasticsearch (if not already connected).
	client, err := opensearch.GetClient()
	if err != nil {
		return nil, models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("connect: %w", err))
	}

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
		godump.Dump(req.GetRequest("GET"))
		return nil, opensearch.GenerateAPIError("search", resp)
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

// WithSort option sets the sort order of search results.
// func WithSort(options ...string) SearchOption {
// 	return func(req *opensearchapi.SearchBody) {
// 		sortCombos := make([]opensearchapi.SortCombinations, 0, len(options))
// 		for option := range slices.Values(options) {
// 			sortCombos = append(sortCombos, opensearchapi.NewSortCombinationsFromOptions(option))
// 		}
// 		sort := opensearchapi.NewSortFromArray(sortCombos)
// 		req.Sort = &sort
// 	}
// }

// WithQuery option sets the query used to find search results.
func WithQuery(options ...QueryOption) SearchOption {
	return func(req *opensearchapi.SearchReq) {
		query := &opensearchapi.CommonQueryDSLQueryContainer{}
		for option := range slices.Values(options) {
			option(query)
		}
		req.Body.Query = query
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

func (r *SearchResponse) IsSuccess() bool {
	if resp := r.Inspect().Response; resp != nil {
		return resp.StatusCode > 199 && resp.StatusCode < 300
	}
	return false
}

func (r *SearchResponse) Error() string {
	return opensearch.GenerateAPIError("search", r).Error()
}

// ExtractSourceFromHits loops through the given hits array and extracts the `_source` field of each document as type
// `T`, returning the document sources as an array `[]T`. If there was an issue extracting any source, it will also
// return a non-nil error containing details. The sort value of the last hit will also be returned, which can be used
// for pagination if needed.
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

// Search performs a _search request to find documents matching the given query.
func SearchAll(
	ctx context.Context,
	index string,
	paginationSize int,
	queries []QueryOption,
	options ...SearchOption,
) (*SearchAllResponse, error) {
	client, err := opensearch.GetClient()
	if err != nil {
		return nil, models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("connect: %w", err))
	}

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

		// Set additional options.
		for option := range slices.Values(options) {
			option(loopReq)
		}

		loopResp, err := client.Search(ctx, loopReq)
		if err != nil {
			godump.Dump(loopReq.GetRequest("GET"))
			resp.errors = append(resp.errors, opensearch.GenerateAPIError("search", loopResp))
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
	var hasErrorResp bool
	for resp := range slices.Values(r.Responses) {
		if resp := resp.Inspect().Response; resp != nil {
			if resp.StatusCode > 399 {
				hasErrorResp = true
			}
		}
	}
	return hasErrorResp
}

func (r *SearchAllResponse) IsSuccess() bool {
	if len(r.errors) > 0 {
		return false
	}
	var hasErrorResp bool
	for resp := range slices.Values(r.Responses) {
		if resp := resp.Inspect().Response; resp != nil {
			if resp.StatusCode < 199 || resp.StatusCode > 300 {
				hasErrorResp = true
			}
		}
	}
	return !hasErrorResp
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

func Count(ctx context.Context, index string, options ...CountOption) (int64, error) {
	// Connect to elasticsearch (if not already connected).
	client, err := opensearch.GetClient()
	if err != nil {
		return 0, models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("connect: %w", err))
	}

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
		return 0, opensearch.GenerateAPIError("search", resp)
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
