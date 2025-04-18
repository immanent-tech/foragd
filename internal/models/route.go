// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"maps"
	"net/http"
	"net/url"
	"sync"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
)

type Route struct {
	path    string
	method  string
	filters *Filters
	sync.Mutex
	attributes templ.Attributes
}

func (r *Route) AddAttribute(key string, value any) {
	r.Lock()
	defer r.Unlock()
	r.attributes[key] = value
}

func (r *Route) RemoveAttribute(key string) {
	r.Lock()
	defer r.Unlock()
	delete(r.attributes, key)
}

func (r *Route) SetAttributes(attributes templ.Attributes) {
	r.Lock()
	defer r.Unlock()
	if r.attributes != nil {
		maps.Copy(r.attributes, attributes)
	} else {
		r.attributes = attributes
	}
}

func (r *Route) GetAttributes() templ.Attributes {
	switch r.method {
	case http.MethodPut:
		r.AddAttribute("hx-put", r.String())
	case http.MethodDelete:
		r.AddAttribute("hx-delete", r.String())
	case http.MethodPost:
		r.AddAttribute("hx-post", r.String())
	case http.MethodGet:
		fallthrough
	default:
		r.AddAttribute("hx-get", r.String())
	}
	return r.attributes
}

func (r *Route) SetFeedIDs(feedIDs ...FeedID) {
	r.filters.Feeds = feedIDs
}

func (r *Route) SetCategories(categories ...Category) {
	r.filters.Categories = categories
}

func (r *Route) UnsetCategories() {
	r.filters.Categories = nil
}

func (r *Route) SetView(view View) {
	r.filters.View = view
}

func (r *Route) SetSortBy(sortBy SortBy) {
	r.filters.SortBy = sortBy
}

func (r *Route) SetSortOrder(sortOrder SortOrder) {
	r.filters.SortOrder = sortOrder
}

func (r *Route) URL() *url.URL {
	rte, err := url.Parse(r.path)
	if err != nil {
		rte, _ = url.Parse("/")
	}
	if r.filters != nil {
		rte.RawQuery = r.filters.ToQueryParams().Encode()
	}
	return rte
}

func (r *Route) String() string {
	return r.URL().String()
}

func NewRouteFromCtx(ctx context.Context) *Route {
	route := &Route{
		path:       chi.RouteContext(ctx).RoutePattern(),
		attributes: make(templ.Attributes),
	}
	filters := FiltersFromCtx(ctx)
	route.filters = &filters
	return route
}

func NewRoute(path string, filters *Filters) *Route {
	route := &Route{
		path:       path,
		filters:    filters,
		attributes: make(templ.Attributes),
	}
	return route
}
