// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package retriever

import (
	"slices"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"

	"github.com/immanent-tech/foragd/providers/elastic/query"
)

// RRFOption is a functional option applied to an RRF retriever.
type RRFOption func(*types.RRFRetriever)

// WithChildRetrievers option appends the given child retrievers to the RRF retriever definition.
func WithChildRetrievers(retrievers ...types.RRFRetrieverEntry) RRFOption {
	return func(r *types.RRFRetriever) {
		r.Retrievers = retrievers
	}
}

// WithRankWindowSize option sets the size of the individual result sets per query. A higher value will
// improve result relevance at the cost of performance. The final ranked result set is pruned down to the search
// request’s size. rank_window_size must be greater than or equal to size and greater than or equal to 1. Defaults to
// 10.
//
// https://www.elastic.co/docs/reference/elasticsearch/rest-apis/retrievers/rrf-retriever
func WithRankWindowSize(size int) RRFOption {
	return func(r *types.RRFRetriever) {
		if size > 0 {
			r.RankWindowSize = &size
		}
	}
}

// WithQueryFilters option defines a (boolean) query that will be applied to filter results from each individual child
// retriever.
func WithQueryFilters(filter query.Option) RRFOption {
	return func(r *types.RRFRetriever) {
		r.Filter = append(r.Filter, *query.Build(filter))
	}
}

// WithStandardRetriever option defines a standard retriever as a RFF child retriever.
func WithStandardRetriever(name string, options ...query.Option) types.RRFRetrieverEntry {
	return &types.RetrieverContainer{
		Standard: &types.StandardRetriever{
			Name_: &name,
			Query: query.Build(options...),
		},
	}
}

// Option is a functional option applied to a retriever definition.
type Option func(*types.RetrieverContainer)

// WithReciprocalRankFusionRetriever option adds an RRF retriever.
func WithReciprocalRankFusionRetriever(options ...RRFOption) Option {
	return func(rc *types.RetrieverContainer) {
		// Build the RRF retriever.
		retriever := &types.RRFRetriever{}
		for option := range slices.Values(options) {
			option(retriever)
		}
		// Add to the container.
		rc.Rrf = retriever
	}
}
