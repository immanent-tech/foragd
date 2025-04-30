// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package templates

import "github.com/a-h/templ"

// DefaultKeywords are the default keywords to insert in a "keywords" <meta> tag.
var DefaultKeywords = []string{"feeds", "atom", "jsonfeed", "rss", "feed reader", "news", "current affairs"}

// DefaultPageTitle is the default <title> tag value if none is set.
const DefaultPageTitle = "Go Feed Me"

// Layout represents a page layout. It has methods to issue a full or partial render of the layout.
type Layout interface {
	PartialLayout
	FullRender() templ.Component
}

// PartialLayout represents partial page content layout. It has a method to render the content. PartialLayout is usually
// called for a HTMX response where a full page load is not required.
type PartialLayout interface {
	PartialRender() templ.Component
}
