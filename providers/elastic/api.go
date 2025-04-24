// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"

	"github.com/elastic/go-elasticsearch/v8/typedapi"
)

// InternalPaginationCount defines the number of docs to retrieve in a pagination request.
const InternalPaginationCount = 1000

// API is an object that provides access to the Elasticsearch API.
type API struct {
	*typedapi.API
}

// GetAPI returns the raw API object.
func (a *API) GetAPI() *typedapi.API {
	return a.API
}

type Link interface {
	Handle(ctx context.Context)
}

type Chain[T any] interface {
	Execute(ctx context.Context) (T, error)
}

type HandlerFunc func(Link) Link

type HandlerChain[T any] struct {
	chain []HandlerFunc
}

// func (c HandlerChain[T]) Execute(ctx context.Context) (T, error) {
// 	for i := range c.chain {
// 		c = c.chain[len(c.chain)-1-i](c)
// 	}

// 	return c
// }

// func NewHandlerChain(constructors ...HandlerFunc) HandlerChain {
// 	return HandlerChain{append(([]HandlerFunc)(nil), constructors...)}
// }
