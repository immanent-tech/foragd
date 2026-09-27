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

	"github.com/immanent-tech/go-base/logging"

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
	options ...Option[*search.Search],
) (*SearchResponse[O], error) {
	// Connect to elasticsearch (if not already connected).
	if err := Connect(); err != nil {
		return nil, fmt.Errorf("connect to elasticsearch: %w", err)
	}

	searchOptions := []Option[*search.Search]{
		WithIndex[*search.Search](index),
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
	options ...Option[*search.Search],
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
		searchOpts := []Option[*search.Search]{
			WithIndex[*search.Search](index),
			WithQuery[*search.Search](query),
			WithSize[*search.Search](paginationSize),
			WithDocSorting[*search.Search](),
			WithSearchAfter[*search.Search](searchAfter...),
			WithTrackHits[*search.Search](false),
		}
		searchOpts = append(searchOpts, options...)
		resp, err := Search[O](ctx, index, searchOpts...)
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
		slog.Int("loops", loops+1),
	)
	return allResults, nil
}

// NewSearchRequest creates a new search request with the given options.
func NewSearchRequest(ctx context.Context, options ...Option[*search.Search]) *search.Search {
	req := api.Search()
	req = WithHeader[*search.Search](ReqIDHeader, middleware.GetReqID(ctx))(req)
	for _, option := range options {
		option(req)
	}
	return req
}
