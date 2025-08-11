// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package templates

import (
	"slices"
	"strings"

	"github.com/a-h/templ"
)

var ContentFragment = FragmentKey{}

type FragmentKey struct{}

// DefaultPageTitle is the default <title> tag value if none is set.
var DefaultPageTitle = "Go Feed Me"

// Page represents a single HTML page.
type Page struct {
	Head        Head
	BodyContent templ.Component
}

// NewPage creates a new page with the given title and options.
func NewPage(title string, options ...PageOption) templ.Component {
	page := &Page{}
	page.Head.Title = title

	for option := range slices.Values(options) {
		option(page)
	}

	return page.render()
}

// PageOption is a functional option to apply to a page.
type PageOption func(*Page)

// WithDescription option sets the page description.
func WithDescription(description string) PageOption {
	return func(p *Page) {
		p.Head.AddMetaTag("description", description)
	}
}

// WithKeywords option sets the page keywords.
func WithKeywords(keywords ...string) PageOption {
	return func(p *Page) {
		p.Head.AddMetaTag("keywords", strings.Join(keywords, ","))
	}
}

func WithBodyContent(content templ.Component) PageOption {
	return func(p *Page) {
		p.BodyContent = content
	}
}
