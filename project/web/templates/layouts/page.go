// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package layouts

import (
	"strings"

	"github.com/a-h/templ"
)

type PageContent interface {
	templ.Component
}

// PageProps represents a full HTML page.
type PageProps struct {
	Title         string
	CustomHeaders []templ.Component
	Content       PageContent
}

// Option represents a generic option that can be applied to a component.
type Option[T any] func(T)

// WithPageKeywords adds the list of keywords to the "keywords" meta tag in the page
// header.
func WithPageKeywords(keywords ...string) Option[*PageProps] {
	return func(p *PageProps) {
		p.CustomHeaders = append(p.CustomHeaders,
			pageMetaTag("keywords", strings.Join(keywords, ",")))
	}
}

// WithPageDescription adds the given description to the "description" meta tage in
// the page header.
func WithPageDescription(description string) Option[*PageProps] {
	return func(p *PageProps) {
		p.CustomHeaders = append(p.CustomHeaders,
			pageMetaTag("description", description))
	}
}

// WithPageContent defines the main body content of the page. It takes a list of
// templ.Components that will be rendered, in the given order in the "body" of
// the page.
func WithPageContent(content PageContent) Option[*PageProps] {
	return func(p *PageProps) {
		p.Content = content
	}
}

func BuildPage(title string, options ...Option[*PageProps]) *PageProps {
	page := &PageProps{
		Title: title,
	}

	for _, option := range options {
		option(page)
	}

	return page
}
