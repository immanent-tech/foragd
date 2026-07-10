// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package htmlx

import (
	"slices"
	"strings"

	"golang.org/x/net/html"
)

// CleanRedditHTML will remove the janky table format that some reddit posts are contained within.
func CleanRedditHTML(content string) string {
	if IsHTML(content) {
		doc, err := html.Parse(strings.NewReader(content))
		if err != nil {
			return content
		}

		row := FindHTMLNode(doc, "tr")
		if row == nil {
			return content
		}

		cells := FindAllHTMLNodes(row, "td")
		if cells == nil {
			return content
		}

		var str strings.Builder
		for cell := range slices.Values(cells) {
			if err := html.Render(&str, cell); err != nil {
				return content
			}
		}

		return str.String()
	}

	return content
}
