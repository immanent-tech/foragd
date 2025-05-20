// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package templates

import (
	"github.com/a-h/templ"
	"github.com/joshuar/go-templ-daisyui/attributes"
)

// ContentID is the id attribute for the main content area.
var (
	ContentID = attributes.ID("content")
	ErrorID   = attributes.ID("error")
)

// DefaultKeywords are the default keywords to insert in a "keywords" <meta> tag.
var DefaultKeywords = []string{"feeds", "atom", "jsonfeed", "rss", "feed reader", "news", "current affairs"}

// DefaultPageTitle is the default <title> tag value if none is set.
const DefaultPageTitle = "Go Feed Me"

// Layout represents the layout of content on a page. It has a Template() method that returns a templ.Component that
// renders the page layout.
type Layout interface {
	Template() templ.Component
}
