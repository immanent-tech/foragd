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
type Option func(*reindex.Reindex)

// NewReindexOperation creates a new reindex operation from the given source to given destination, with the given options.
//
// https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-reindex
func NewReindexOperation(api *elasticsearch.TypedClient, src *types.ReindexSource, dest *types.ReindexDestination, options ...Option) *reindex.Reindex {
	// Create the base operation.
	reidx := api.Reindex().Source(src).Dest(dest)
	// Apply options.
	for option := range slices.Values(options) {
		option(reidx)
	}
	// Return operation.
	return reidx
}

// NewSource sets up a new reindex source.
func NewSource(src string) *types.ReindexSource {
	return &types.ReindexSource{
		Index: []string{src},
	}
}

// NewDest sets up a new reindex destination.
func NewDest(dest string) *types.ReindexDestination {
	return &types.ReindexDestination{
		Index:  dest,
		OpType: &optype.Create,
	}
}
