// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package partials

import (
	"maps"
	"sync"

	"github.com/a-h/templ"
)

const (
	ImgProxyKey  contextKey = "imgproxy_key"
	ImgProxySalt contextKey = "imgproxy_salt"
)

type contextKey string

type Properties struct {
	attributes templ.Attributes
	classes    []string
	mu         sync.Mutex
}

func NewProperties() *Properties {
	return &Properties{
		attributes: make(templ.Attributes),
		classes:    make([]string, 0),
	}
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
}

func (p *Properties) Attributes() templ.Attributes {
	return p.attributes
}

func (p *Properties) Classes() []string {
	return p.classes
}

type PropertiesOption func(*Properties)

func WithAttribute(key string, value any) PropertiesOption {
	return func(p *Properties) {
		p.setAttribute(key, value)
	}
}

func WithMergeAttributes(attributes templ.Attributes) PropertiesOption {
	return func(p *Properties) {
		p.mergeAttributes(attributes)
	}
}

func WithClasses(classes ...string) PropertiesOption {
	return func(p *Properties) {
		p.setClasses(classes...)
	}
}
