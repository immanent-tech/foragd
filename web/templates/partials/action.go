// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package partials

import (
	"net/http"
	"slices"
	"sync"

	"github.com/a-h/templ"
	"github.com/joshuar/go-templ-daisyui/actions/button"
	"github.com/joshuar/go-templ-daisyui/navigation/link"
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
	AttrHXPost    = "hx-post"
	AttrHXGet     = "hx-get"
	AttrHXPut     = "hx-put"
)

type DisplayType int

type Action struct {
	Path          string            `json:"path"`
	Method        string            `json:"method"`
	Vars          map[string]string `json:"vars"`
	display       DisplayType
	buttonOptions []button.Option
	linkOptions   []link.Option
	sync.Mutex    `json:"-"`
}

func NewAction(path string, display DisplayType, options ...ActionOption) *Action {
	link := &Action{
		Path:    path,
		Method:  http.MethodGet,
		Vars:    make(map[string]string),
		display: display,
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

func ActionButtonOptions(options ...button.Option) ActionOption {
	return func(a *Action) {
		a.buttonOptions = options
	}
}

func ActionLinkOptions(options ...link.Option) ActionOption {
	return func(a *Action) {
		a.linkOptions = options
	}
}

func ActionValues(values map[string]string) ActionOption {
	return func(a *Action) {
		a.setVar(AttrHXVals, GenerateHXVals(values))
	}
}

func ActionParams(value string) ActionOption {
	return func(a *Action) {
		a.setVar(AttrHXParams, value)
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
	case http.MethodPut:
		attrs[AttrHXPut] = l.Path
	}
	return attrs
}
