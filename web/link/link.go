// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package link

import (
	"net/http"
	"slices"
	"sync"

	"github.com/a-h/templ"

	"github.com/joshuar/go-feed-me/web/templates/partials"
)

const (
	AttrHXTarget  = "hx-target"
	AttrHXPushURL = "hx-push-url"
	AttrHXVals    = "hx-vals"
	AttrHXGet     = "hx-get"
	AttrHXPost    = "hx-post"
)

type InternalLink struct {
	Path       string            `json:"path"`
	Method     string            `json:"method"`
	Vars       map[string]string `json:"vars"`
	sync.Mutex `json:"-"`
}

func New(path string, options ...LinkOption) *InternalLink {
	link := &InternalLink{
		Path:   path,
		Method: http.MethodGet,
		Vars:   make(map[string]string),
	}
	// Set default variables.
	link.setVar(AttrHXTarget, partials.ContentID.Target())
	// Apply options.
	for option := range slices.Values(options) {
		option(link)
	}

	return link
}

type LinkOption func(*InternalLink)

func WithMethod(method string) LinkOption {
	return func(l *InternalLink) {
		l.Method = method
	}
}

func WithTarget(target string) LinkOption {
	return func(il *InternalLink) {
		il.setVar(AttrHXTarget, target)
	}
}

func WithPushURL() LinkOption {
	return func(il *InternalLink) {
		il.setVar(AttrHXPushURL, "true")
	}
}

func WithValues(values map[string]string) LinkOption {
	return func(il *InternalLink) {
		il.setVar(AttrHXVals, partials.GenerateHXVals(values))
	}
}

func (l *InternalLink) setVar(key, value string) {
	l.Lock()
	defer l.Unlock()
	l.Vars[key] = value
}

func (l *InternalLink) generateAttrs() templ.Attributes {
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
