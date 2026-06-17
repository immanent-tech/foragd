// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package htmx

import (
	"maps"
	"net/http"
	"slices"
	"sync"

	"github.com/a-h/templ"
)

// Attributes represents htmx hx-* attributes applied to an element.
type Attributes struct {
	attributes templ.Attributes
	mu         sync.Mutex
}

// NewAttributes creates a Attributes object for an element with the given options.
func NewAttributes(options ...AttributesOption) *Attributes {
	props := &Attributes{
		attributes: make(templ.Attributes, len(options)),
	}
	for option := range slices.Values(options) {
		if option == nil {
			continue
		}
		option(props)
	}
	return props
}

// GetAttributes returns the hx-* attributes as a templ.Attributes.
func (p *Attributes) GetAttributes() templ.Attributes {
	return p.attributes
}

// HasAttribute returns a boolean indicating whether there is an attribute with the given key.
func (p *Attributes) HasAttribute(key string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.attributes[key]
	return ok
}

// SetAttribute sets an attribute with the given key to the given value.  Any existing value is overridden.
func (p *Attributes) SetAttribute(key string, value any) {
	p.setAttribute(key, value)
}

func (p *Attributes) setAttribute(key string, value any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.attributes == nil {
		p.attributes = make(templ.Attributes)
	}
	p.attributes[key] = value
}

func (p *Attributes) mergeAttributes(attributes templ.Attributes) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.attributes == nil {
		p.attributes = make(templ.Attributes)
	}
	maps.Copy(p.attributes, attributes)
}

// AttributesOption is a functional option to set Properties.
type AttributesOption func(*Attributes)

// WithHXMethod sets the method attribute (i.e., hx-get, hx-post, etc.) to the given value.
func WithHXMethod(method, value string) AttributesOption {
	return func(a *Attributes) {
		if value == "" {
			return
		}
		switch method {
		case http.MethodGet:
			a.setAttribute("hx-get", value)
		case http.MethodPost:
			a.setAttribute("hx-post", value)
		case http.MethodDelete:
			a.setAttribute("hx-delete", value)
		case http.MethodPut:
			a.setAttribute("hx-put", value)
		}
	}
}

// WithHXSwap sets the hx-swap attribute.
func WithHXSwap(value string) AttributesOption {
	return func(a *Attributes) {
		if value == "" {
			return
		}
		a.setAttribute("hx-swap", value)
	}
}

// WithHXInclude sets the hx-include attribute.
func WithHXInclude(value string) AttributesOption {
	return func(a *Attributes) {
		if value == "" {
			return
		}
		a.setAttribute("hx-include", value)
	}
}

// WithHXTrigger sets the hx-trigger attribute.
func WithHXTrigger(value string) AttributesOption {
	return func(a *Attributes) {
		if value == "" {
			return
		}
		a.setAttribute("hx-trigger", value)
	}
}

// WithHXTarget sets the hx-target attribute.
func WithHXTarget(target string) AttributesOption {
	return func(a *Attributes) {
		if target == "" {
			return
		}
		a.setAttribute("hx-target", target)
	}
}

// WithHXVals sets the hx-vals attribute. It can handle a string value directly or a map of values which will get
// marshaled into a JSON string representation.
func WithHXVals(vals any) AttributesOption {
	return func(a *Attributes) {
		switch values := vals.(type) {
		case string:
			a.setAttribute("hx-vals", values)
		default:
			marshaled, err := templ.JSONString(values)
			if err != nil {
				return
			}
			a.setAttribute("hx-vals", marshaled)
		}
	}
}

// WithHXPushURL sets hx-push-url to true.
func WithHXPushURL() AttributesOption {
	return func(a *Attributes) {
		a.setAttribute("hx-push-url", true)
	}
}

// WithHXReplaceURL sets hx-replace-url to true.
func WithHXReplaceURL() AttributesOption {
	return func(a *Attributes) {
		a.setAttribute("hx-replace-url", true)
	}
}
