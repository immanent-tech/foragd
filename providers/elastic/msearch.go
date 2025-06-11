// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"log/slog"
	"slices"

	"github.com/elastic/go-elasticsearch/v8/typedapi"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/msearch"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

type MSearchOptions struct {
	Query *types.Query
	Sort  []types.SortCombinationsVariant
}

func (mso *MSearchOptions) GenerateSortCombination() []types.SortCombinations {
	combos := make([]types.SortCombinations, 0, len(mso.Sort))
	for sort := range slices.Values(mso.Sort) {
		combos = append(combos, sort.SortCombinationsCaster())
	}
	return combos
}

// query *types.Query, sort []types.SortCombinations
func WithSearch[V MSearchRequest[*msearch.Msearch]](index string, searchOptions *MSearchOptions) Option[V] {
	return func(t V) {
		if searchOptions == nil {
			return
		}

		hdr := types.NewMultisearchHeader()
		hdr.Index = append(hdr.Index, index)

		search := types.NewMultisearchBody()
		search.Query = searchOptions.Query
		search.Sort = searchOptions.GenerateSortCombination()

		err := t.AddSearch(*hdr, *search)
		if err != nil {
			slog.Warn("error occurred", slog.Any("error", err))
		}
	}
}

func WithRequestID[T any, V SearchRequestCommon[T]](id string) Option[V] {
	return func(t V) {
		if id != "" {
			t.Header(ReqIDHeader, id)
		}
	}
}

func NewMSearchRequest(api *typedapi.API, options ...any) *msearch.Msearch {
	req := api.Msearch()

	for option := range slices.Values(options) {
		switch value := option.(type) {
		case Option[SearchRequestCommon[*msearch.Msearch]]:
			value(req)
		case Option[*msearch.Msearch]:
			value(req)
		default:
			slog.Warn("ignoring option")
		}
	}

	return req
}
