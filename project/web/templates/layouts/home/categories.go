// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"slices"

	"github.com/joshuar/go-feed-me/internal/api"
	"github.com/joshuar/go-feed-me/internal/models"
)

type FeedCategoryFilter struct {
	name   models.Category
	route  *api.Route
	active bool
}

func NewCategoryFilter(name models.Category, active bool, route *api.Route) FeedCategoryFilter {
	return FeedCategoryFilter{
		name:   name,
		active: active,
		route:  route,
	}
}

func BuildCategoryFilters(filters *api.Filters, categoryCounts []api.CategoryCount, path string) []FeedCategoryFilter {
	categoryFilters := make([]FeedCategoryFilter, 0, len(categoryCounts))

	for _, category := range categoryCounts {
		var (
			route  *api.Route
			active bool
		)

		switch path {
		case "/home/feeds":
			route = buildShowFeedsRoute(filters)
		case "/home/items":
			route = buildShowItemsRoute(filters)
		}

		route.SetCategories()

		if len(filters.GetCategories()) > 0 {
			if slices.Contains(filters.GetCategories(), category.Category) {
				// This category is being used as a filter.
				active = true
				// Remove the categories param.
				route.UnsetCategories()
			} else {
				// Add the category as a param.
				route.SetCategories(category.Category)
			}
		} else {
			// Add the category as a param.
			route.SetCategories(category.Category)
		}

		categoryFilters = append(categoryFilters, NewCategoryFilter(category.Category, active, route))
	}

	return categoryFilters
}
