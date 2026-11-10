/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package search

import (
	"slices"

	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"
)

// SearchParamsOption is a functional option to set query string parameters to a search request.
type SearchParamsOption func(*opensearchapi.SearchParams)

// WithSearchParameters option sets query string parameters on the search.
func WithSearchParameters(options ...SearchParamsOption) SearchOption {
	params := &opensearchapi.SearchParams{}

	for option := range slices.Values(options) {
		option(params)
	}

	return func(req *opensearchapi.SearchReq) {
		req.Params = params
	}
}

// WithSortParams option sets sort parameters on the search.
func WithSortParams(sort ...string) SearchParamsOption {
	return func(sp *opensearchapi.SearchParams) {
		sp.Sort = append(sp.Sort, sort...)
	}
}

// WithSeqNoPrimaryTerm option ensures the results returns sequence number and primary term of the last modification of each
// hit.
func WithSeqNoPrimaryTerm(value bool) SearchParamsOption {
	return func(sp *opensearchapi.SearchParams) {
		sp.SeqNoPrimaryTerm = &value
	}
}
