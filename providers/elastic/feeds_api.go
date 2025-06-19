// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/msearch"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/go-chi/chi/v5/middleware"
	slogctx "github.com/veqryn/slog-context"

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
		WithSearchID(middleware.GetReqID(ctx)),
		WithSearchIndex(index),
		WithSearchQueryOptions(query),
		WithSearchSize(0),
		WithAggregations(aggregations...),
	)

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, parseError(err)
	}

	return resp, nil
}

func (a *API) MultiSearch(ctx context.Context, feedsSearch, itemsSearch *query.MSearchOptions) (models.Feeds, models.Items, error) {
	subscriptionsIndex := FeedsIndexFromCtx(ctx)
	if subscriptionsIndex == "" {
		return nil, nil, errors.Join(ErrUpdateFailed, ErrFetchCtx)
	}
	itemsIndex := ItemsIndexFromCtx(ctx)
	if itemsIndex == "" {
		return nil, nil, errors.Join(ErrUpdateFailed, ErrFetchCtx)
	}

	req := NewMSearchRequest(a.GetAPI(),
		WithSearch[*msearch.Msearch](subscriptionsIndex, feedsSearch),
		WithSearch[*msearch.Msearch](itemsIndex, itemsSearch),
		WithRequestID[*msearch.Msearch, SearchRequestCommon[*msearch.Msearch]](middleware.GetReqID(ctx)),
	)

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", ErrReqFailed, err)
	}

	results := make(map[string]*types.MultiSearchItem)

	for idx, r := range []string{"subscriptions", "items"} {
		switch res := resp.Responses[idx].(type) {
		case *types.MultiSearchItem:
			if res.Hits.Total.Value == 0 {
				continue
			}
			results[r] = res
		case types.ErrorResponseBase:
		default:
		}
	}

	var (
		feeds    models.Feeds
		items    models.Items
		warnings error
	)
	if results["subscriptions"] != nil {
		feeds, _, warnings = ExtractSourceFromHits[*models.Feed](results["subscriptions"].Hits.Hits)
		if warnings != nil {
			slogctx.FromCtx(ctx).Warn("Problem extracting hits.", slog.Any("err", err))
		}
	}
	if results["items"] != nil {
		items, _, warnings = ExtractSourceFromHits[*models.Item](results["items"].Hits.Hits)
		if warnings != nil {
			slogctx.FromCtx(ctx).Warn("Problem extracting hits.", slog.Any("err", err))
		}
	}

	return feeds, items, nil
}
