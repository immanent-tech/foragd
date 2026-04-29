// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"

	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/immanent-tech/foragd/providers/elastic/query"
)

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
