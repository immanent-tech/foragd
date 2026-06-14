// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package html

import (
	"strings"

	"golang.org/x/net/html"
)

// checkMediumSignals walks the HTML tree and counts Medium-specific markers.
func checkMediumSignals(n *html.Node) int {
	count := 0

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "meta":
				name := attrVal(n, "name")
				content := attrVal(n, "content")
				property := attrVal(n, "property")

				// Primary signal: Medium's generator tag
				if strings.EqualFold(name, "generator") &&
					strings.EqualFold(content, "Medium") {
					count += 10 // strong signal
				}

				// Medium's Apollo client state hint
				if strings.EqualFold(name, "medium-lite-url") {
					count += 5
				}

				// Open Graph site name often set to "Medium"
				if strings.EqualFold(property, "og:site_name") &&
					strings.EqualFold(content, "Medium") {
					count++
				}

			case "link":
				href := attrVal(n, "href")
				// Medium serves assets from miro.medium.com
				if strings.Contains(href, "miro.medium.com") ||
					strings.Contains(href, "medium.com") {
					count++
				}

			case "script":
				src := attrVal(n, "src")
				// Medium's JS bundles come from medium.com
				if strings.Contains(src, "medium.com") ||
					strings.Contains(src, "miro.medium.com") {
					count++
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return count
}

func attrVal(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}
