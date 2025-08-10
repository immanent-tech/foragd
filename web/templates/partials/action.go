// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package partials

import (
	"encoding/json"
	"net/http"
	"slices"
	"sync"

	"github.com/a-h/templ"
	components "github.com/joshuar/go-templ-daisyui"
)

const (
	DisplayActionAsLink DisplayType = iota
	DisplayActionAsButton
)

const (
	AttrHXTarget  = "hx-target"
	AttrHXPushURL = "hx-push-url"
	AttrHXVals    = "hx-vals"
	AttrHXSwap    = "hx-swap"
	AttrHXParams  = "hx-params"
	AttrHXInclude = "hx-include"
	AttrHXPost    = "hx-post"
	AttrHXGet     = "hx-get"
	AttrHXPut     = "hx-put"
	AttrHXDelete  = "hx-delete"
)

type DisplayType int

// Action represents a htmx-powered action component.
type Action struct {
	Path       string            `json:"path"`
	Method     string            `json:"method"`
	Vars       map[string]string `json:"vars"`
	display    DisplayType
	classes    *components.Classes
	sync.Mutex `json:"-"`
}

// NewAction will create a new action that will be rendered as the given type, with the given options.
func NewAction(path string, display DisplayType, options ...ActionOption) *Action {
	link := &Action{
		Path:    path,
		Method:  http.MethodGet,
		Vars:    make(map[string]string),
		display: display,
		classes: components.NewClasses(),
	}
	// Set default variables.
	link.setVar(AttrHXTarget, ContentID.Target())
	// Apply options.
	for option := range slices.Values(options) {
		option(link)
	}

	return link
}

// ActionOption is a functional option to apply to an action.
type ActionOption func(*Action)

// ActionMethod option defines the HTTP method (hx-{get,post,put} etc.) to use for the action. If this option is not
// specified, it will default to a 'hx-get'.
func ActionMethod(method string) ActionOption {
	return func(a *Action) {
		a.Method = method
	}
}

// ActionTarget option sets the 'hx-target' attribute to the given target.
func ActionTarget(target string) ActionOption {
	return func(a *Action) {
		a.setVar(AttrHXTarget, target)
	}
}

// ActionSwap option defines the 'hx-swap' behaviour.
func ActionSwap(swap string) ActionOption {
	return func(a *Action) {
		a.setVar(AttrHXSwap, swap)
	}
}

// ActionInclude option defines the 'hx-include' option.
func ActionInclude(include string) ActionOption {
	return func(a *Action) {
		a.setVar(AttrHXInclude, include)
	}
}

// ActionPushURL option sets 'hx-push-url' to true.
func ActionPushURL() ActionOption {
	return func(a *Action) {
		a.setVar(AttrHXPushURL, "true")
	}
}

// ActionValues option sets the content of the 'hx-vals' attribute.
func ActionValues(values map[string]string) ActionOption {
	return func(a *Action) {
		a.setVar(AttrHXVals, GenerateHXVals(values))
	}
}

// ActionParams option sets the content of the 'hx-params' attribute.
func ActionParams(value string) ActionOption {
	return func(a *Action) {
		a.setVar(AttrHXParams, value)
	}
}

// ActionAttribute sets an attribute on the action element.
func ActionAttribute(key, value string) ActionOption {
	return func(a *Action) {
		a.setVar(key, value)
	}
}

// ActionClasses option sets the given classes on the action.
func ActionClasses(classes ...components.Class) ActionOption {
	return func(a *Action) {
		a.classes.Add(classes...)
	}
}

// ActionHyperScript option defines hyperscript code to be assigned to the component.
func ActionHyperScript(script string) ActionOption {
	return func(a *Action) {
		a.setVar("_", script)
	}
}

// Attributes will render all attributes of the action as a templ.Attributes map.
func (l *Action) Attributes() templ.Attributes {
	attrs := make(templ.Attributes)
	for k, v := range l.Vars {
		attrs[k] = v
	}
	switch l.Method {
	case http.MethodGet:
		attrs[AttrHXGet] = l.Path
	case http.MethodPost:
		attrs[AttrHXPost] = l.Path
	case http.MethodPut:
		attrs[AttrHXPut] = l.Path
	case http.MethodDelete:
		attrs[AttrHXDelete] = l.Path
	}
	return attrs
}

func (l *Action) setVar(key, value string) {
	l.Lock()
	defer l.Unlock()
	l.Vars[key] = value
}

// GenerateHXVals generates a JSON-formatted object containing the given key-value pairs suitable for use as a hx-vals attribute.
// See also: https://htmx.org/attributes/hx-vals/
func GenerateHXVals(values map[string]string) string {
	data, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return string(data)
}
