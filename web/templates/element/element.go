// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package element

import (
	"maps"
	"strings"
	"sync"

	"github.com/a-h/templ"

	"github.com/immanent-tech/foragd/models"
)

// Element represents any HTML element that can have attributes and classes.
type Element struct {
	mu         sync.Mutex
	ID         models.ElementID
	Attributes templ.Attributes
	Classes    []string
}

func New() *Element {
	return &Element{
		Attributes: make(templ.Attributes),
		Classes:    make([]string, 0),
	}
}

func (e *Element) GetID() string {
	return e.ID.String()
}

func (e *Element) GetTarget() string {
	return e.ID.Target()
}

func (e *Element) SetID(id string) {
	e.ID = models.ElementID(id)
}

func (e *Element) SetAttribute(key string, value any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Attributes[key] = value
}

func (e *Element) MergeAttributes(attributes templ.Attributes) {
	e.mu.Lock()
	defer e.mu.Unlock()
	maps.Copy(attributes, e.Attributes)
}

func (e *Element) HasAttribute(key string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	_, ok := e.Attributes[key]
	return ok
}

func (e *Element) GetAttributes() templ.Attributes {
	return e.Attributes
}

func (e *Element) AddClasses(classes ...string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Classes = append(e.Classes, classes...)
}

func (e *Element) GetClasses() string {
	return strings.Join(e.Classes, " ")
}

// Option is a functional option applied to an Element.
type Option func(*Element)

// WithClasses option applies the given classes to the Element.
func WithClasses(classes ...string) Option {
	return func(e *Element) {
		e.AddClasses(classes...)
	}
}

// WithAttribute option applies the given attribute to the Element.
func WithAttribute(key string, value any) Option {
	return func(e *Element) {
		e.SetAttribute(key, value)
	}
}

// WithAttributes option applies the set of attributes to the Element. It will merge the set with any existing elements
// (duplicate existing attributes will be overridden).
func WithAttributes(attributes templ.Attributes) Option {
	return func(e *Element) {
		if attributes != nil {
			e.MergeAttributes(attributes)
		}
	}
}

func WithID(id string) Option {
	return func(e *Element) {
		e.SetID(id)
	}
}
