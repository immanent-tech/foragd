// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic/aggregations"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
)

// ItemsAggregation performs a search aggregation (i.e., only aggregations returned) on feed items with the given query
// options. It returns the raw search response.
func (e *API) ItemsAggregation(ctx context.Context, query query.Option, aggregations ...aggregations.Aggregation) (*search.Response, *models.Response) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, parseError(ErrFetchCtx)
	}

	req := NewSearchRequest(e.GetAPI(),
		WithRequestID[*search.Search, SearchAPIRequest](middleware.GetReqID(ctx)),
		WithIndex[*search.Search, SearchAPIRequest](index),
		WithQueryOptions[*search.Search, SearchAPIRequest](query),
		WithSize[*search.Search, SearchAPIRequest](0),
		WithAggregations[*search.Search, SearchAPIRequest](aggregations...),
	)

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, parseError(err)
	}

	return resp, nil
}
