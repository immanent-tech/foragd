// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package layouts

import (
	"strings"

	"github.com/a-h/templ"

	"github.com/joshuar/go-feed-me/web/templates"
)

// Props contains the customisable properties for a Page.
type Props templates.Page

// PageOption is an option that can be applied to a Page.
type PageOption templates.Option[*Props]

type PageContent interface {
	templ.Component
}

// WithPageKeywords adds the list of keywords to the "keywords" meta tag in the page
// header.
func WithPageKeywords(keywords ...string) PageOption {
	return func(p *Props) {
		p.Headers = append(p.Headers,
			pageMetaTag("keywords", strings.Join(keywords, ",")))
	}
}

// WithPageDescription adds the given description to the "description" meta tage in
// the page header.
func WithPageDescription(description string) PageOption {
	return func(p *Props) {
		p.Headers = append(p.Headers,
			pageMetaTag("description", description))
	}
}

// WithPageContent defines the main body content of the page. It takes a list of
// templ.Components that will be rendered, in the given order in the "body" of
// the page.
func WithPageContent(content PageContent) PageOption {
	return func(p *Props) {
		p.Content = content
	}
}

// BuildPage builds a page object from the given options.
func BuildPage(title string, options ...PageOption) *Props {
	page := &Props{
		Title: title,
	}

	for _, option := range options {
		option(page)
	}

	return page
}
