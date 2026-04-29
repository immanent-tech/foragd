// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"slices"

	elasticsearch "github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/get"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/mget"
	"github.com/go-chi/chi/v5/middleware"
)

type GetRequest struct {
	*get.Get
}

// NewGetRequest creates a new get object with the given options.
func NewGetRequest(
	ctx context.Context,
	api *elasticsearch.TypedClient,
	index string,
	id string,
	options ...func(*GetRequest),
) *GetRequest {
	req := &GetRequest{
		Get: api.Get(index, id),
	}

	WithHeader[*GetRequest](ReqIDHeader, middleware.GetReqID(ctx))(req)

	for option := range slices.Values(options) {
		option(req)
	}

	return req
}

func (r *GetRequest) SetHeader(key, value string) {
	r.Get = r.Header(key, value)
}

type MGetRequest struct {
	*mget.Mget
}

// NewMGetRequest creates a new mget object with the given options.
func NewMGetRequest(ctx context.Context, api *elasticsearch.TypedClient, options ...func(*MGetRequest)) *MGetRequest {
	req := &MGetRequest{
		Mget: api.Mget(),
	}

	WithHeader[*MGetRequest](ReqIDHeader, middleware.GetReqID(ctx))(req)

	for option := range slices.Values(options) {
		option(req)
	}

	return req
}

func (r *MGetRequest) SetHeader(key, value string) {
	r.Mget = r.Header(key, value)
}

func (r *MGetRequest) SetIndex(index string) {
	r.Mget = r.Index(index)
}

func (r *MGetRequest) SetIDs(ids ...string) {
	r.Mget = r.Ids(ids...)
}
