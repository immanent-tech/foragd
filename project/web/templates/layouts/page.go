// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package layouts

import (
	"strings"

	"github.com/a-h/templ"

	"github.com/joshuar/go-feed-me/web/templates/partials/meta"
)

// Page represents a full HTML page.
type Page struct {
	Title         string
	CustomHeaders []templ.Component
	Content       []templ.Component
}

// Option represents a generic option that can be applied to a component.
type Option[T any] func(T) T

// WithPageKeywords adds the list of keywords to the "keywords" meta tag in the page
// header.
func WithPageKeywords(keywords ...string) Option[Page] {
	return func(p Page) Page {
		p.CustomHeaders = append(p.CustomHeaders,
			meta.Tag("keywords", strings.Join(keywords, ",")))

		return p
	}
}

// WithPageDescription adds the given description to the "description" meta tage in
// the page header.
func WithPageDescription(description string) Option[Page] {
	return func(p Page) Page {
		p.CustomHeaders = append(p.CustomHeaders,
			meta.Tag("description", description))

		return p
	}
}

// WithPageContent defines the main body content of the page. It takes a list of
// templ.Components that will be rendered, in the given order in the "body" of
// the page.
func WithPageContent(content ...templ.Component) Option[Page] {
	return func(p Page) Page {
		p.Content = append(p.Content, content...)

		return p
	}
}

// NewPage creates a new page with the given options.
func NewPage(title string, options ...Option[Page]) Page {
	page := Page{
		Title: title,
	}

	for _, option := range options {
		page = option(page)
	}

	return page
}
