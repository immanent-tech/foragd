// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package api

import (
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/a-h/templ"

	"github.com/joshuar/go-feed-me/internal/forms"
	"github.com/joshuar/go-feed-me/internal/models"
)

const (
	Get    HTMXMethod = iota // hx-get
	Put                      // hx-put
	Post                     // hx-post
	Delete                   // hx-delete

	UnknownErrorPath = "/error"
)

var DefaultRouteMethod = http.MethodGet

//go:generate go tool golang.org/x/tools/cmd/stringer -type=HTMXMethod -linecomment -output route.gen.go
type HTMXMethod int

func (r *Route) GetParams() url.Values {
	return r.url.Query()
}

func (r *Route) GetCategoriesParam() []models.Category {
	categories := r.url.Query()["categories"]
	return categories
}

func (r *Route) GetViewParam() View {
	return View(r.url.Query().Get("view"))
}

func (r *Route) GetCountParam() Count {
	if count, err := strconv.Atoi(r.url.Query().Get("count")); err != nil {
		return count
	}

	return 10
}

// String shows the route URL as a string.
func (r *Route) String() string {
	return r.url.String()
}

// ShowAttributes will return the htmx attributes for the route.
func (r *Route) Attributes() templ.Attributes {
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

// Rebuild allows rebuilding an existing Route with different options. All
// existing params and attributes are cleared and only the path and method are
// retained.
func (r *Route) Rebuild(options ...RouteOption) {
	// clear existing attributes
	clear(r.attributes)
	// clear existing params
	r.url.RawQuery = ""

	for _, option := range options {
		option(r)
	}
}

// SetView returns a copy of Route with the view query parameter set to the
// given value.
func (r *Route) SetView(view View) {
	params := WithViewParam(view)(r.url.Query())
	r.url.RawQuery = params.Encode()
}

func (r *Route) SetSort(sort Sort) {
	params := WithSortParam(sort)(r.url.Query())
	r.url.RawQuery = params.Encode()
}

func (r *Route) SetCategories(categories ...models.Category) {
	params := WithCategoriesParam(categories...)(r.url.Query())
	r.url.RawQuery = params.Encode()
}

func (r *Route) UnsetCategories() {
	params := r.url.Query()
	params.Del("categories")
	r.url.RawQuery = params.Encode()
}

func (r *Route) SetFeeds(feedIDs ...models.FeedID) {
	params := WithFeedsParam(feedIDs...)(r.url.Query())
	r.url.RawQuery = params.Encode()
}

func (r *Route) SetItems(itemIDs ...models.ItemID) {
	params := WithItemsParam(itemIDs...)(r.url.Query())
	r.url.RawQuery = params.Encode()
}

func (r *Route) SetMark(mark Mark) {
	params := WithMarkParam(mark)(r.url.Query())
	r.url.RawQuery = params.Encode()
}

func (r *Route) SetPagination(pagination Pagination) {
	params := WithPaginationParam(pagination)(r.url.Query())
	r.url.RawQuery = params.Encode()
}

func (r *Route) AddAttribute(key, value string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, found := r.attributes[key]; !found {
		r.attributes[key] = value
	}
}

func (r *Route) RemoveAttribute(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.attributes, key)
}

func (r *Route) SetAttributes(attributes templ.Attributes) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.attributes != nil {
		maps.Copy(r.attributes, attributes)
	} else {
		r.attributes = attributes
	}
}

func (r *Route) SetMethod(method string) {
	r.method = &method
}

// RouteOption is a functional option to customize a Route.
type RouteOption Option[*Route]

// WithFeedsParam option replaces any existing models.FeedID filters with the given list.
func WithFeedsParam(ids ...models.FeedID) ParamsOption {
	return func(v url.Values) url.Values {
		v.Set("feeds", strings.Join(ids, ","))
		return v
	}
}

// WithItemsParam option replaces any existing models.ItemID filters with the given list.
func WithItemsParam(ids ...models.ItemID) ParamsOption {
	return func(v url.Values) url.Values {
		v.Set("items", strings.Join(ids, ","))
		return v
	}
}

// WithCategoriesParam option replaces any existing Categories filters with the given list.
func WithCategoriesParam(categories ...models.Category) ParamsOption {
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

func WithSortParam(sort Sort) ParamsOption {
	return func(v url.Values) url.Values {
		sortValues, err := forms.EncodeForm(&sort)
		if err != nil {
			slog.Warn("Problem encoding sort values.", slog.Any("error", err))
		}
		maps.Copy(v, sortValues)
		return v
	}
}

func WithSubPath(path string) RouteOption {
	return func(a *Route) {
		a.url = *a.url.JoinPath(path)
	}
}

func WithParams(options ...ParamsOption) RouteOption {
	return func(route *Route) {
		params := route.url.Query()

		for _, option := range options {
			params = option(params)
		}

		route.url.RawQuery = params.Encode()
	}
}

func WithAttribute(key, value string) RouteOption {
	return func(r *Route) {
		r.AddAttribute(key, value)
	}
}

func WithAttributes(attrs templ.Attributes) RouteOption {
	return func(r *Route) {
		r.SetAttributes(attrs)
	}
}

func WithMethod(method string) RouteOption {
	return func(r *Route) {
		r.method = &method
	}
}

// BuildRoute creates a new Route with params defined by the given options.
func BuildRoute(from any, options ...RouteOption) *Route {
	route := &Route{
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
