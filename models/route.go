// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"sync"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
)

// Route is a routing path or endpoint on the service. Typical usage is for generating a path for some user action in
// the UI.
type Route struct {
	path    string
	method  string
	filters *Filters
	sync.Mutex
	attributes templ.Attributes
}

// AddAttribute will add the given attribute to the route.
func (r *Route) AddAttribute(key string, value any) {
	r.Lock()
	defer r.Unlock()
	r.attributes[key] = value
}

// RemoveAttribute will remove the given attribute from the route.
func (r *Route) RemoveAttribute(key string) {
	r.Lock()
	defer r.Unlock()
	delete(r.attributes, key)
}

// SetAttributes sets the given attributes on the route. It will merge the given attributes with existing ones.
func (r *Route) SetAttributes(attributes templ.Attributes) {
	r.Lock()
	defer r.Unlock()
	if r.attributes != nil {
		maps.Copy(r.attributes, attributes)
	} else {
		r.attributes = attributes
	}
}

// GetAttributes will return the attributes of the route, which can be used as component attributes.
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

// SetFeedIDs sets the feed filters for the route.
func (r *Route) SetFeedIDs(feedIDs ...FeedID) {
	r.filters.Feeds = feedIDs
}

// SetCategories sets the category filters for the route.
func (r *Route) SetCategories(categories ...Category) {
	r.filters.Categories = categories
}

// UnsetCategories removes all category filters from the route.
func (r *Route) UnsetCategories() {
	r.filters.Categories = nil
}

// SetView sets the view filter of the route.
func (r *Route) SetView(view View) {
	r.filters.View = view
}

// SetSortBy sets the sort by filter of the route.
func (r *Route) SetSortBy(sortBy SortBy) {
	r.filters.SortBy = sortBy
}

// SetSortOrder sets the sort order filter of the route.
func (r *Route) SetSortOrder(sortOrder SortOrder) {
	r.filters.SortOrder = sortOrder
}

// URL will generate a return the route as a url.URL object.
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

// RouteOption is a functional option for a Route.
type RouteOption Option[*Route]

// WithAttributes option sets the given attributes on the route.
func WithAttributes(attributes templ.Attributes) RouteOption {
	return func(r *Route) {
		r.SetAttributes(attributes)
	}
}

// NewRouteFromCtx generates a route from details within the context.
func NewRouteFromCtx(ctx context.Context, options ...RouteOption) *Route {
	route := &Route{
		path:       chi.RouteContext(ctx).RoutePattern(),
		attributes: make(templ.Attributes),
	}
	filters := FiltersFromCtx(ctx)
	route.filters = &filters

	for option := range slices.Values(options) {
		option(route)
	}

	return route
}

// NewRoute generates a route from the given path and filters.
func NewRoute(path string, filters *Filters, options ...RouteOption) *Route {
	route := &Route{
		path:       path,
		filters:    filters,
		attributes: make(templ.Attributes),
	}

	for option := range slices.Values(options) {
		option(route)
	}

	return route
}
