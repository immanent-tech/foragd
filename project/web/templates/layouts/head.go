// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package layouts

import (
	"strings"

	"github.com/a-h/templ"
)

// HeadProps holds the properties for creating a page <head> element.
type HeadProps struct {
	title        string
	extraHeaders []templ.Component
}

// HeadOption is an option that can be applied to the <head> element of a page.
type HeadOption func(*HeadProps)

func WithAdditionalHeaders(headers ...templ.Component) HeadOption {
	return func(head *HeadProps) {
		head.extraHeaders = headers
	}
}

// WithPageKeywords adds the list of keywords to the "keywords" meta tag in the page
// header.
func WithPageKeywords(keywords ...string) HeadOption {
	return func(head *HeadProps) {
		head.extraHeaders = append(head.extraHeaders,
			pageMetaTag("keywords", strings.Join(keywords, ",")))
	}
}

// WithPageDescription adds the given description to the "description" meta tage in
// the page header.
func WithPageDescription(description string) HeadOption {
	return func(head *HeadProps) {
		head.extraHeaders = append(head.extraHeaders,
			pageMetaTag("description", description))
	}
}

// BuildHeader builds a head object from the given options.
func BuildHeader(title string, options ...HeadOption) *HeadProps {
	head := &HeadProps{}

	// Set the title (or use a default).
	if title == "" {
		head.title = "Go Feed Me"
	} else {
		head.title = title
	}
	// Set any additional properties.
	for _, option := range options {
		option(head)
	}

	return head
}
