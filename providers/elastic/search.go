// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/go-chi/chi/v5/middleware"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/logging"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/results"
)

// SearchResponse represents the results of a search request. In addition to exposing the raw API response, the object
// contains marshaled results and pagination values.
type SearchResponse[O any] struct {
	*search.Response

	Results    []O
	Pagination []types.FieldValue
}

// Search performs a _search request to find documents matching the given query.
func Search[O any](
	ctx context.Context,
	index string,
	query query.Option,
	options ...func(*SearchRequest),
) (*SearchResponse[O], error) {
	// Connect to elasticsearch (if not already connected).
	if err := Connect(); err != nil {
		return nil, fmt.Errorf("connect to elasticsearch: %w", err)
	}

	searchOptions := []func(*SearchRequest){
		WithIndex[*SearchRequest](index),
		WithQueryOptions[*SearchRequest](query),
	}
	searchOptions = append(searchOptions, options...)
	req := NewSearchRequest(ctx, searchOptions...)

	resp := &SearchResponse[O]{}
	var err error
	resp.Response, err = req.Do(ctx)
	if err != nil {
		return resp, fmt.Errorf("search: %w", err)
	}

	var warnings error

	resp.Results, resp.Pagination, warnings = results.ExtractSourceFromHits[O](resp.Hits.Hits)
	if warnings != nil {
		slogctx.FromCtx(ctx).WarnContext(ctx, "Some docs could not be extracted.",
			slog.Any("warnings", warnings))
	}

	return resp, nil
}

// SearchAll performs a paginated search request to retrieve *all* documents matching the given query. Unlike Search, it
// does not stop when the request hits count is reached.
func SearchAll[O any](
	ctx context.Context,
	index string,
	query query.Option,
	paginationSize int,
	options ...func(*SearchRequest),
) ([]O, error) {
	// Connect to elasticsearch (if not already connected).
	if err := Connect(); err != nil {
		return nil, fmt.Errorf("connect to elasticsearch: %w", err)
	}

	if paginationSize == 0 {
		paginationSize = 1000
	}
	allResults := make([]O, 0)
	var searchAfter []types.FieldValueVariant

	// Loop until we've paginated through all results.
	var loops int
	for {
		searchOpts := []func(*SearchRequest){
			WithIndex[*SearchRequest](index),
			WithQueryOptions[*SearchRequest](query),
			WithSize(paginationSize),
			WithDocSorting(),
			WithSearchAfter(searchAfter...),
			WithTrackTotalHits(false),
		}
		searchOpts = append(searchOpts, options...)
		resp, err := Search[O](ctx, index, query, searchOpts...)
		if err != nil {
			return nil, fmt.Errorf("search all: %w", err)
		}
		pagination, err := EncodePagination[string](resp.Pagination)
		if err != nil {
			return nil, fmt.Errorf("search all: encode pagination: %w", err)
		}
		searchAfter, err = DecodePagination(&pagination)
		if err != nil {
			return nil, fmt.Errorf("search all: decode pagination: %w", err)
		}

		allResults = append(allResults, resp.Results...)
		// Stop if the number of hits is less than the search size (i.e., last set of hits).
		if len(resp.Results) < paginationSize {
			break
		}
		loops++
	}
	slogctx.FromCtx(ctx).Log(ctx, logging.LevelTrace, "Paginated search finished.",
		slog.Int("loops", loops),
	)
	return allResults, nil
}

// SearchRequest represents an elastic search request.
type SearchRequest struct {
	*search.Search
}

// NewSearchRequest creates a new search request with the given options.
func NewSearchRequest(ctx context.Context, options ...func(*SearchRequest)) *SearchRequest {
	req := &SearchRequest{
		Search: api.Search(),
	}

	WithHeader[*SearchRequest](ReqIDHeader, middleware.GetReqID(ctx))(req)

	for _, option := range options {
		option(req)
	}

	return req
}

func (r *SearchRequest) SetHeader(key, value string) {
	r.Search = r.Header(key, value)
}

func (r *SearchRequest) SetIndex(index string) {
	r.Search = r.Index(index)
}

func (r *SearchRequest) SetQueryOptions(options ...query.Option) {
	if query := query.Build(options...); query != nil {
		r.Search = r.Query(query)
	}
}

func (r *SearchRequest) SetAggregations(aggs map[string]types.Aggregations) {
	r.Search = r.Aggregations(aggs)
}

func (r *SearchRequest) SetSize(size int) {
	r.Search = r.Size(size)
}

func (r *SearchRequest) SetSearchAfter(values ...types.FieldValueVariant) {
	r.Search = r.SearchAfter(values...)
}

func (r *SearchRequest) SetSort(sort ...types.SortCombinationsVariant) {
	r.Search = r.Sort(sort...)
}

func (r *SearchRequest) SetFields(fields ...types.FieldAndFormatVariant) {
	r.Search = r.Fields(fields...)
}

func (r *SearchRequest) SetTrackTotalHits(value bool) {
	r.Search = r.TrackTotalHits(TrackHits(value))
}

func (r *SearchRequest) SetCollapseOn(collapse *types.FieldCollapse) {
	r.Search = r.Collapse(collapse)
}
