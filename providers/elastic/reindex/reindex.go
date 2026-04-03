// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package reindex

import (
	"slices"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/reindex"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/optype"
)

// Option is a functional option for a reindex operation.
type Option func(*reindex.Reindex) *reindex.Reindex

// NewReindexOperation creates a new reindex operation from the given source to given destination, with the given options.
//
// https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-reindex
func NewReindexOperation(
	api *elasticsearch.TypedClient,
	src *types.ReindexSource,
	dest *types.ReindexDestination,
	options ...Option,
) *reindex.Reindex {
	// Create the base operation.
	reidx := api.Reindex().Source(src).Dest(dest)
	// Apply options.
	for option := range slices.Values(options) {
		reidx = option(reidx)
	}
	// Return operation.
	return reidx
}

// WithRequestsPerSecond option sets the requests per second rate limit for the reindex operation. By default, there is
// no rate limit.
func WithRequestsPerSecond(rps string) Option {
	return func(r *reindex.Reindex) *reindex.Reindex {
		return r.RequestsPerSecond(rps)
	}
}

// WithSlices option sets the number of slices the reindex will be divided into. By default no slicing is performed. If
// set to `auto`, Elasticsearch chooses the number of slices to use. This setting will use one slice per shard, up to a
// certain limit. If there are multiple sources, it will choose the number of slices based on the index or backing index
// with the smallest number of shards.
func WithSlices(slices string) Option {
	return func(r *reindex.Reindex) *reindex.Reindex {
		return r.Slices(slices)
	}
}

// WithRequireAlias option sets whether the reindex destination is required to be an index alias.
func WithRequireAlias(value bool) Option {
	return func(r *reindex.Reindex) *reindex.Reindex {
		return r.RequireAlias(value)
	}
}

// NewSource sets up a new reindex source.
func NewSource(src string) *types.ReindexSource {
	return &types.ReindexSource{
		Index: []string{src},
	}
}

// NewDest sets up a new reindex destination.
func NewDest(indexName, pipeline string) *types.ReindexDestination {
	dest := &types.ReindexDestination{
		Index:  indexName,
		OpType: &optype.Create,
	}
	if pipeline != "" {
		dest.Pipeline = &pipeline
	}
	return dest
}
