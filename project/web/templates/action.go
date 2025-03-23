// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package templates

import (
	"maps"
	"net/http"
	"net/url"
	"sync"

	"github.com/a-h/templ"
)

// Action represents a HTMX action that an be assigned to a Component.
type Action struct {
	mu         sync.Mutex
	method     string `validate:"required"`
	path       string `validate:"required"`
	parameters url.Values
	attributes templ.Attributes
}

// Path returns the URL path (with query parameters) as a string.
func (a *Action) Path() string {
	return a.path + "?" + a.parameters.Encode()
}

// Attributes returns the Action attributes.
func (a *Action) Attributes() templ.Attributes {
	switch a.method {
	case http.MethodPut:
		a.AddAttribute("hx-put", a.Path())
	case http.MethodDelete:
		a.AddAttribute("hx-delete", a.Path())
	case http.MethodPost:
		a.AddAttribute("hx-post", a.Path())
	case http.MethodGet:
		fallthrough
	default:
		a.AddAttribute("hx-get", a.Path())
	}
	return a.attributes
}

func (a *Action) AddAttribute(key, value string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, found := a.attributes[key]; !found {
		a.attributes[key] = value
	}
}

func (a *Action) RemoveAttribute(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.attributes, key)
}

func (a *Action) SetAttributes(attributes templ.Attributes) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.attributes != nil {
		maps.Copy(a.attributes, attributes)
	} else {
		a.attributes = attributes
	}
}

func (a *Action) AddParameter(key, value string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.parameters.Set(key, value)
}

func (a *Action) RemoveParameter(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.parameters.Del(key)
}

// ActionOption is a functional option to customize a Action.
type ActionOption func(*Action)

// BuildAction creates a new Action with the given options.
func BuildAction(path string, options ...ActionOption) *Action {
	action := &Action{
		path:       path,
		method:     http.MethodGet,
		attributes: make(templ.Attributes),
		mu:         sync.Mutex{},
	}

	for _, option := range options {
		option(action)
	}

	return action
}

// WithMethod option will ensure the action uses the given method. If this
// option is not supplied, the action will default to a "get" action.
func WithMethod(method string) ActionOption {
	return func(action *Action) {
		switch method {
		case http.MethodPut:
			action.method = method
		case http.MethodDelete:
			action.method = method
		case http.MethodPost:
			action.method = method
		case http.MethodGet:
			fallthrough
		default:
			action.method = http.MethodGet
		}
	}
}

// WihtQueryParams options assigns the given query parameters (as url.Values) to
// the Action, which will be used when generating the Action's route.
func WithQueryParams(values url.Values) ActionOption {
	return func(action *Action) {
		if values != nil {
			action.parameters = values
		}
	}
}

// WithAttributes option assigns the given Attributes to the Action, which will
// be added as attributes to the Action's route.
func WithAttributes(attributes templ.Attributes) ActionOption {
	return func(action *Action) {
		if attributes != nil {
			action.SetAttributes(attributes)
		}
	}
}
