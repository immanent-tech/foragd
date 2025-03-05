// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"maps"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/a-h/templ"
)

const (
	Get    HTMXMethod = iota // hx-get
	Put                      // hx-put
	Post                     // hx-post
	Delete                   // hx-delete

	UnknownErrorPath = "/error"
)

var DefaultRouteMethod = http.MethodGet

//go:generate go tool golang.org/x/tools/cmd/stringer -type=HTMXMethod -linecomment -output routing.gen.go
type HTMXMethod int

func (r *APIRoute) GetParams() url.Values {
	return r.url.Query()
}

func (r *APIRoute) GetCategoriesParam() []Category {
	categories := r.url.Query()["categories"]
	return categories
}

func (r *APIRoute) GetViewParam() View {
	return View(r.url.Query().Get("view"))
}

func (r *APIRoute) GetCountParam() Count {
	if count, err := strconv.Atoi(r.url.Query().Get("count")); err != nil {
		return count
	}

	return 10
}

// String shows the route URL as a string.
func (r *APIRoute) String() string {
	return r.url.String()
}

// ShowAttributes will return the htmx attributes for the route.
func (r *APIRoute) Attributes() templ.Attributes {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch *r.method {
	case http.MethodGet:
		r.attributes[Get.String()] = r.url.String()
	case http.MethodPost:
		r.attributes[Post.String()] = r.url.String()
	case http.MethodDelete:
		r.attributes[Delete.String()] = r.url.String()
	case http.MethodPut:
		r.attributes[Put.String()] = r.url.String()
	}

	return r.attributes
}

// Rebuild allows rebuilding an existing APIRoute with different options. All
// existing params and attributes are cleared and only the path and method are
// retained.
func (r *APIRoute) Rebuild(options ...RouteOption) {
	// clear existing attributes
	clear(r.attributes)
	// clear existing params
	r.url.RawQuery = ""

	for _, option := range options {
		option(r)
	}
}

// SetView returns a copy of APIRoute with the view query parameter set to the
// given value.
func (r *APIRoute) SetView(view View) *APIRoute {
	params := WithViewParam(view)(r.url.Query())
	r.url.RawQuery = params.Encode()

	return r
}

func (r *APIRoute) SetCategories(categories ...Category) *APIRoute {
	params := WithCategoriesParam(categories...)(r.url.Query())
	r.url.RawQuery = params.Encode()

	return r
}

func (r *APIRoute) UnsetCategories() *APIRoute {
	params := r.url.Query()
	params.Del("categories")
	r.url.RawQuery = params.Encode()

	return r
}

func (r *APIRoute) SetFeeds(feedIDs ...FeedID) {
	params := WithFeedsParam(feedIDs...)(r.url.Query())
	r.url.RawQuery = params.Encode()
}

func (r *APIRoute) SetItems(itemIDs ...ItemID) {
	params := WithItemsParam(itemIDs...)(r.url.Query())
	r.url.RawQuery = params.Encode()
}

func (r *APIRoute) SetMark(mark Mark) {
	params := WithMarkParam(mark)(r.url.Query())
	r.url.RawQuery = params.Encode()
}

func (r *APIRoute) SetPagination(pagination Pagination) {
	params := WithPaginationParam(pagination)(r.url.Query())
	r.url.RawQuery = params.Encode()
}

func (r *APIRoute) AddAttribute(key, value string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, found := r.attributes[key]; !found {
		r.attributes[key] = value
	}
}

func (r *APIRoute) RemoveAttribute(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.attributes, key)
}

func (r *APIRoute) SetAttributes(attributes templ.Attributes) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.attributes != nil {
		maps.Copy(r.attributes, attributes)
	} else {
		r.attributes = attributes
	}
}

func (r *APIRoute) SetMethod(method string) {
	r.method = &method
}

// RouteOption is a functional option to customize a APIRoute.
type RouteOption Option[*APIRoute]

// WithFeedsParam option replaces any existing FeedID filters with the given list.
func WithFeedsParam(ids ...FeedID) ParamsOption {
	return func(v url.Values) url.Values {
		v.Set("feeds", strings.Join(ids, ","))
		return v
	}
}

// WithItemsParam option replaces any existing ItemID filters with the given list.
func WithItemsParam(ids ...ItemID) ParamsOption {
	return func(v url.Values) url.Values {
		v.Set("items", strings.Join(ids, ","))
		return v
	}
}

// WithCategoriesParam option replaces any existing Categories filters with the given list.
func WithCategoriesParam(categories ...Category) ParamsOption {
	return func(v url.Values) url.Values {
		if len(categories) > 0 {
			v.Set("categories", strings.Join(categories, ","))
		}

		return v
	}
}

// WithoutCategoriesParam option removes any existing Categories filters.
func WithoutCategoriesParam() ParamsOption {
	return func(v url.Values) url.Values {
		v.Del("categories")
		return v
	}
}

func WithViewParam(view View) ParamsOption {
	return func(v url.Values) url.Values {
		v.Set("view", string(view))
		return v
	}
}

func WithMarkParam(mark Mark) ParamsOption {
	return func(v url.Values) url.Values {
		v.Set("mark", string(mark))
		return v
	}
}

func WithCountParam(count int) ParamsOption {
	return func(v url.Values) url.Values {
		v.Set("count", strconv.Itoa(count))
		return v
	}
}

func WithPaginationParam(pagination Pagination) ParamsOption {
	return func(v url.Values) url.Values {
		v.Set(string(ParamPagination), pagination)
		return v
	}
}

func WithSubPath(path string) RouteOption {
	return func(a *APIRoute) {
		a.url = *a.url.JoinPath(path)
	}
}

func WithParams(options ...ParamsOption) RouteOption {
	return func(route *APIRoute) {
		params := route.url.Query()

		for _, option := range options {
			params = option(params)
		}

		route.url.RawQuery = params.Encode()
	}
}

func WithAttribute(key, value string) RouteOption {
	return func(r *APIRoute) {
		r.AddAttribute(key, value)
	}
}

func WithAttributes(attrs templ.Attributes) RouteOption {
	return func(r *APIRoute) {
		r.SetAttributes(attrs)
	}
}

func WithMethod(method string) RouteOption {
	return func(r *APIRoute) {
		r.method = &method
	}
}

// BuildRoute creates a new APIRoute with params defined by the given options.
func BuildRoute(from any, options ...RouteOption) *APIRoute {
	route := &APIRoute{
		attributes: make(templ.Attributes),
		method:     &DefaultRouteMethod,
		mu:         sync.Mutex{},
	}

	var (
		parsed *url.URL
		err    error
	)

	switch value := from.(type) {
	case *url.URL:
		parsed, err = url.Parse(value.String())
	case string:
		parsed, err = url.Parse(value)
	}

	if err != nil {
		parsed, _ = url.Parse(UnknownErrorPath) //nolint:errcheck // should never fail.
	}

	route.url = *parsed

	for _, option := range options {
		option(route)
	}

	return route
}
