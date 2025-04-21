// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package watcher

import (
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/joshuar/go-feed-me/internal/platforms/elastic"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/query"
)

type SearchInputOption elastic.Option[*types.SearchInput]

// SearchInput sets the watch input to a search request with the given
// options.
func SearchInput(options ...SearchInputOption) *types.SearchInput {
	input := types.NewSearchInput()

	for _, option := range options {
		option(input)
	}

	return input
}

// WithIndicies defines which indicies the search input will query.
func WithIndices(index ...string) SearchInputOption {
	return func(input *types.SearchInput) {
		input.Request.Indices = index
	}
}

// WithQueryOptions defines the query options for the search input.
func WithQueryOptions(options ...query.Option) SearchInputOption {
	return func(input *types.SearchInput) {
		if query := query.Build(options...); query != nil {
			input.Request.Body.Query = *query
		}
	}
}
