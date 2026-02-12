// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"fmt"
	"slices"
	"sort"

	"github.com/immanent-tech/foragd/validation"
)

// CommonCategoryFilters is slice of categories that are so general or inclusive that they are ultimately useless for
// queries or aggregations.
//
// Regular expressions are supported. See:
//
// https://www.elastic.co/docs/reference/query-languages/query-dsl/regexp-syntax
var CommonCategoryFilters = []string{
	"Post",
	"Posts",
	"News",
	"Article",
	"Articles",
	"Links",
	"Uncategorized",
	"Featured",
	"Headline",
	"Home Feed",
}

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

func (c CategoryCounts) GetCategories() []Category {
	var categories []Category
	for count := range slices.Values(c) {
		categories = append(categories, count.Category)
	}
	return categories
}

// Limit returns the n top Categories from the list of
// CategoryCounts.
func (c CategoryCounts) Limit(count int) CategoryCounts {
	if len(c) == 0 {
		return nil
	}
	c.Sort()
	if count > len(c) {
		count = len(c)
	}
	return c[:count]
}

func (r *ListSubscriptionCategoriesRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("list subscription categories request is invalid: %w", err)
	}
	return nil
}

func (r *ListSubscriptionCategoriesRequest) Sanitise() error {
	return nil
}
