// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"encoding/base64"
	"slices"
	"sort"
)

// Categories is a slice of categories.
type Categories []Category

// HasCategory is a convienience function to check if the given category is in the slice of categories.
func (c Categories) HasCategory(category Category) bool {
	return slices.Contains(c, category)
}

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
func (c CategoryCounts) GetTopCategories(count int) []CategoryCount {
	if len(c) == 0 {
		return nil
	}
	c.Sort()
	if count > len(c) {
		count = len(c)
	}
	return c[:count]
}

// CategoryID turns a category into a string suitable to use as an ID for the category. This is done by encoding the
// category as a base64 string, which is safe to use in HTML.
func CategoryID(category Category) string {
	return base64.StdEncoding.EncodeToString([]byte(category))
}
