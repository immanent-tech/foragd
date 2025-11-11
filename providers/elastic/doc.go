// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"log/slog"
	"slices"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/deletebyquery"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/get"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/mget"
)

// NewDeleteByQueryRequest creates a new delete by query request that will operate on the given index with the given
// options.
func NewDeleteByQueryRequest(api *elasticsearch.TypedClient, index string, options ...any) *deletebyquery.DeleteByQuery {
	req := api.DeleteByQuery(index)

	for option := range slices.Values(options) {
		switch value := option.(type) {
		case Option[RequestCommon[*deletebyquery.DeleteByQuery]]:
			value(req)
		case Option[RequestWithQuery[*deletebyquery.DeleteByQuery]]:
			value(req)
		case Option[*deletebyquery.DeleteByQuery]:
			value(req)
		default:
			slog.Warn("ignoring option")
		}
	}

	return req
}

// NewGetRequest creates a new get doc request that will operate on the given index with the given options.
func NewGetRequest(api *elasticsearch.TypedClient, index, id string, options ...any) *get.Get {
	req := api.Get(index, id)
	for option := range slices.Values(options) {
		switch value := option.(type) {
		case Option[RequestCommon[*get.Get]]:
			value(req)
		case Option[*get.Get]:
			value(req)
		default:
			slog.Warn("ignoring option")
		}
	}

	return req
}

// MgetRequest represents an Elasticsearch _mget request.
type MgetRequest interface {
	RequestCommon[*mget.Mget]
	RequestWithIDs[*mget.Mget]
	RequestWithIndex[*mget.Mget]
}

// NewMGetRequest creates a new mget object with the given options.
func NewMGetRequest(api *elasticsearch.TypedClient, options ...Option[MgetRequest]) *mget.Mget {
	req := api.Mget()
	for option := range slices.Values(options) {
		option(req)
	}

	return req
}
