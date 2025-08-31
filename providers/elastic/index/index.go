// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package index

import (
	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/deletebyquery"

	"github.com/immanent-tech/go-feed-me/providers/elastic"
	"github.com/immanent-tech/go-feed-me/providers/elastic/query"
)

type DeleteByQueryOption elastic.Option[*deletebyquery.DeleteByQuery]

// NewSearchRequest creates a new search request with the given options.
func NewDeleteByQueryRequest(api *elasticsearch.TypedClient, index string, options ...DeleteByQueryOption) *deletebyquery.DeleteByQuery {
	req := api.DeleteByQuery(index)

	for _, option := range options {
		option(req)
	}

	return req
}

// WithSearchQueryOptions adds the given query options (conditions) to the search.
func WithDeleteQueryOptions(options ...query.Option) DeleteByQueryOption {
	return func(dbq *deletebyquery.DeleteByQuery) {
		if query := query.Build(options...); query != nil {
			dbq.Query(query)
		}
	}
}
