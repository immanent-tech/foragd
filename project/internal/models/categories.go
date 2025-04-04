// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"sort"
)

// CategoryCounts is a slice of Categories and their counts.
type CategoryCounts []CategoryCount

// Sort will sort the list of CategoryCounts by the count values.
func (c CategoryCounts) Sort() {
	sort.Slice(c, func(i, j int) bool {
		return c[i].Count > c[j].Count
	})
}

// GetTopCategories returns the n top Categories from the list of
// CategoryCounts.
func (c CategoryCounts) GetTopCategories(n int) []CategoryCount {
	c.Sort()
	return c[:n-1]
}

// // CleanCategories takes an array of Categories and "cleans" them, removing any
// // HTML escape strings for better display.
// func CleanCategory(category Category) Category {
// 	return html.UnescapeString(category.String())
// }

// func CleanCategories(categories ...Category) []Category {
// 	cleaned := make([]Category, 0, len(categories))
// 	for category := range slices.Values(categories) {
// 		cleaned = append(cleaned, html.UnescapeString(safePrinter.Sanitize(category)))
// 	}
// 	return cleaned
// }
