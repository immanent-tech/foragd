// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package layouts

import (
	"github.com/a-h/templ"
)

// Page represents a page on the website.
type Props struct {
	Content templ.Component
	Header  *HeadProps
}

// PageOption is an option that can be applied to a Page.
type PageOption func(*Props)

type PageContent interface {
	templ.Component
}

func WithHeadOptions(title string, options ...HeadOption) PageOption {
	return func(page *Props) {
		page.Header = BuildHeader(title, options...)
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
func BuildPage(options ...PageOption) *Props {
	page := &Props{}

	for _, option := range options {
		option(page)
	}

	return page
}
