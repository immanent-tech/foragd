// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package element

import (
	"maps"
	"slices"
	"sync"

	"github.com/a-h/templ"
)

// Properties represents properties of an element, including its attributes and classes.
type Properties struct {
	attributes templ.Attributes
	classes    []string
	mu         sync.Mutex
}

// NewProperties creates a Properties object for an element with the given options.
func NewProperties(options ...PropertiesOption) *Properties {
	props := &Properties{
		attributes: make(templ.Attributes),
		classes:    make([]string, 0),
	}
	for option := range slices.Values(options) {
		option(props)
	}
	return props
}

func (p *Properties) setAttribute(key string, value any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.attributes == nil {
		p.attributes = make(templ.Attributes)
	}
	p.attributes[key] = value
}

func (p *Properties) mergeAttributes(attributes templ.Attributes) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.attributes == nil {
		p.attributes = make(templ.Attributes)
	}
	maps.Copy(p.attributes, attributes)
}

func (p *Properties) setClasses(class ...string) {
	p.classes = append(p.classes, class...)
	p.classes = slices.Compact(p.classes)
}

// Attributes returns the attributes as a templ.Attributes.
func (p *Properties) Attributes() templ.Attributes {
	return p.attributes
}

// HasAttribute returns a boolean indicating whether there is an attribute with the given key.
func (p *Properties) HasAttribute(key string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.attributes[key]
	return ok
}

// SetAttribute sets an attribute with the given key to the given value.  Any existing value is overridden.
func (p *Properties) SetAttribute(key string, value any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.attributes[key] = value
}

// ID returns the value of the "id" attribute. If there is no id attribute, an empty string is returned.
func (p *Properties) ID() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if id, ok := p.attributes["id"].(string); ok {
		return id
	}
	return ""
}

// Classes returns the classes as a []string.
func (p *Properties) Classes() []string {
	return p.classes
}

// PropertiesOption is a functional option to set Properties.
type PropertiesOption func(*Properties)

// WithAttribute option sets a key-value attribute on the properties. Any existing value is overridden.
func WithAttribute(key string, value any) PropertiesOption {
	return func(p *Properties) {
		p.setAttribute(key, value)
	}
}

// WithClasses option sets the classes on the properties. New values are merged with existing values and any duplicates
// removed.
func WithClasses(classes ...string) PropertiesOption {
	return func(p *Properties) {
		p.setClasses(classes...)
	}
}

// MergeAttributes option merges the given attributes with any existing attributes.
func MergeAttributes(attributes templ.Attributes) PropertiesOption {
	return func(p *Properties) {
		p.mergeAttributes(attributes)
	}
}
