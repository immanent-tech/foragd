// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package partials

import (
	"slices"

	"github.com/a-h/templ"

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
			paramsOptions []api.ParamsOption
			active        bool
		)

		// Base params should include view and count filters.
		paramsOptions = append(paramsOptions,
			api.WithViewParam(filters.View),
			api.WithCountParam(filters.Count),
		)

		if len(filters.GetCategories()) > 0 {
			if slices.Contains(filters.GetCategories(), category.Category) {
				// This category is being used as a filter.
				active = true
				// Remove the categories param.
				paramsOptions = append(paramsOptions, api.WithoutCategoriesParam())
			} else {
				// Add the category as a param.
				paramsOptions = append(paramsOptions, api.WithCategoriesParam(category.Category))
			}
		} else {
			// Add the category as a param.
			paramsOptions = append(paramsOptions, api.WithCategoriesParam(category.Category))
		}

		// Create a route for setting/unsetting this category filter.
		route := api.BuildRoute(path,
			api.WithAttributes(templ.Attributes{
				"hx-target":   "#content",
				"hx-push-url": "true",
			}),
			api.WithParams(paramsOptions...),
		)

		categoryFilters = append(categoryFilters, NewCategoryFilter(category.Category, active, route))
	}

	return categoryFilters
}
