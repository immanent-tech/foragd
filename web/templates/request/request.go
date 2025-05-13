// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package request contains objects and methods for HTMX features in templates.
package request

import (
	"maps"
	"net/http"
	"net/url"
	"slices"
	"sync"

	"github.com/a-h/templ"
)

// Request represents contains properties for defining a HTMX request.
type Request struct {
	sync.Mutex
	path       string
	method     string
	params     url.Values
	attributes templ.Attributes
}

// AddAttribute adds the given (htmx) attribute to the request.
func (a *Request) AddAttribute(key string, value any) {
	a.Lock()
	defer a.Unlock()
	a.attributes[key] = value
}

// SetAttributes sets the given htmx attributes on the request. It will merge the given attributes with existing ones.
func (a *Request) SetAttributes(attributes templ.Attributes) {
	a.Lock()
	defer a.Unlock()
	if a.attributes != nil {
		maps.Copy(a.attributes, attributes)
	} else {
		a.attributes = attributes
	}
}

// URL returns the request as a URL object.
func (a *Request) URL() *url.URL {
	rte, err := url.Parse(a.path)
	if err != nil {
		rte, _ = url.Parse("/")
	}
	rte.RawQuery = a.params.Encode()
	return rte
}

func (a *Request) String() string {
	return a.URL().String()
}

// Attributes returns the request as htmx attributes that can be attached to a template or component.
func (a *Request) Attributes() templ.Attributes {
	switch a.method {
	case http.MethodPut:
		a.AddAttribute("hx-put", a.String())
	case http.MethodDelete:
		a.AddAttribute("hx-delete", a.String())
	case http.MethodPost:
		a.AddAttribute("hx-post", a.String())
	case http.MethodGet:
		fallthrough
	default:
		a.AddAttribute("hx-get", a.String())
	}
	return a.attributes
}

// Option is a functional option for building an request.
type Option func(*Request)

// WithMethod option sets the http method to use for the request. If this option is not specified, the request will
// default to using a GET.
func WithMethod(method string) Option {
	return func(a *Request) {
		a.method = method
	}
}

// WithParams option sets additional URL parameters to the request path. These are merged with any existing params.
func WithParams(params url.Values) Option {
	return func(a *Request) {
		maps.Copy(a.params, params)
	}
}

// WithParam option sets the given URL param on the request.
func WithParam(key string, value string) Option {
	return func(a *Request) {
		a.params.Add(key, value)
	}
}

// WithAttributes option merges the given attributes with any existing attributes..
func WithAttributes(attributes templ.Attributes) Option {
	return func(a *Request) {
		a.SetAttributes(attributes)
	}
}

// BuildRequest creates a new request for the given path with the given options.
func BuildRequest(path string, options ...Option) *Request {
	action := &Request{
		path:   path,
		params: make(url.Values),
	}

	for option := range slices.Values(options) {
		option(action)
	}

	return action
}
