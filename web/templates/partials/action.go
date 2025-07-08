// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package partials

import (
	"net/http"
	"slices"
	"sync"

	"github.com/a-h/templ"
)

const (
	AttrHXTarget  = "hx-target"
	AttrHXPushURL = "hx-push-url"
	AttrHXVals    = "hx-vals"
	AttrHXGet     = "hx-get"
	AttrHXSwap    = "hx-swap"
	AttrHXPost    = "hx-post"

	DisplayAsButton DisplayType = iota
)

type DisplayType int

type Action struct {
	Path       string            `json:"path"`
	Method     string            `json:"method"`
	Vars       map[string]string `json:"vars"`
	display    DisplayType
	sync.Mutex `json:"-"`
}

func NewAction(path string, options ...ActionOption) *Action {
	link := &Action{
		Path:   path,
		Method: http.MethodGet,
		Vars:   make(map[string]string),
	}
	// Set default variables.
	link.setVar(AttrHXTarget, ContentID.Target())
	// Apply options.
	for option := range slices.Values(options) {
		option(link)
	}

	return link
}

type ActionOption func(*Action)

func ActionMethod(method string) ActionOption {
	return func(a *Action) {
		a.Method = method
	}
}

func ActionTarget(target string) ActionOption {
	return func(a *Action) {
		a.setVar(AttrHXTarget, target)
	}
}

func ActionSwap(swap string) ActionOption {
	return func(a *Action) {
		a.setVar(AttrHXSwap, swap)
	}
}

func ActionPushURL() ActionOption {
	return func(il *Action) {
		il.setVar(AttrHXPushURL, "true")
	}
}

func ActionAsButton() ActionOption {
	return func(a *Action) {
		a.display = DisplayAsButton
	}
}

func ActionValues(values map[string]string) ActionOption {
	return func(a *Action) {
		a.setVar(AttrHXVals, GenerateHXVals(values))
	}
}

func ActionHyperScript(script string) ActionOption {
	return func(a *Action) {
		a.setVar("_", script)
	}
}

func (l *Action) setVar(key, value string) {
	l.Lock()
	defer l.Unlock()
	l.Vars[key] = value
}

func (l *Action) generateAttrs() templ.Attributes {
	attrs := make(templ.Attributes)
	for k, v := range l.Vars {
		attrs[k] = v
	}
	switch l.Method {
	case http.MethodGet:
		attrs[AttrHXGet] = l.Path
	case http.MethodPost:
		attrs[AttrHXGet] = l.Path
	}
	return attrs
}
