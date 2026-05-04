// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package mailto provides a method for easily constructing a mailto: string for use in links.
package mailto

import (
	"net/url"
	"slices"
	"strings"
)

// MailTo represents a link that will open the user's mail client, with optionall pre-filled details).
type MailTo struct {
	parts []string
}

// Build creates a new html link with a `mailto:` href attribute.
func Build(to string, options ...Option) string {
	mtl := &MailTo{}
	for option := range slices.Values(options) {
		option(mtl)
	}
	var builder strings.Builder
	builder.WriteString("mailto:")
	builder.WriteString(to)
	if len(mtl.parts) > 0 {
		builder.WriteString("?")
		builder.WriteString(strings.Join(mtl.parts, "&"))
	}

	return builder.String()
}

// Option is a functional option to apply to a mailto: link object.
type Option func(*MailTo)

// WithSubject option adds a subject to the mailto: link.
func WithSubject(subject string) Option {
	return func(mtl *MailTo) {
		mtl.parts = append(mtl.parts, "subject="+url.QueryEscape(subject))
	}
}

// WithBody option adds body text to the mailto: link.
func WithBody(body string) Option {
	return func(mtl *MailTo) {
		mtl.parts = append(mtl.parts, "body="+url.QueryEscape(body))
	}
}
