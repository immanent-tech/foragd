// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package partials

import (
	"slices"

	"github.com/a-h/templ"

	"github.com/joshuar/go-feed-me/internal/models"
)

type FeedCategoryFilter struct {
	name   models.Category
	route  *models.APIRoute
	active bool
}

func NewCategoryFilter(name models.Category, active bool, route *models.APIRoute) FeedCategoryFilter {
	return FeedCategoryFilter{
		name:   name,
		active: active,
		route:  route,
	}
}

func BuildCategoryFilters(filters *models.APIFilters, allCategories []models.CategoryCount, path string) []FeedCategoryFilter {
	categoryFilters := make([]FeedCategoryFilter, 0, len(allCategories))

	for _, category := range allCategories {
		var (
			paramsOptions []models.ParamsOption
			active        bool
		)

		// Base params should include view and count filters.
		paramsOptions = append(paramsOptions,
			models.WithViewParam(filters.View),
			models.WithCountParam(filters.Count),
		)

		if len(filters.GetCategories()) > 0 {
			if slices.Contains(filters.GetCategories(), category.Name) {
				// This category is being used as a filter.
				active = true
				// Remove the categories param.
				paramsOptions = append(paramsOptions, models.WithoutCategoriesParam())
			} else {
				// Add the category as a param.
				paramsOptions = append(paramsOptions, models.WithCategoriesParam(category.Name))
			}
		} else {
			// Add the category as a param.
			paramsOptions = append(paramsOptions, models.WithCategoriesParam(category.Name))
		}

		// Create a route for setting/unsetting this category filter.
		route := models.BuildRoute(path,
			models.WithAttributes(templ.Attributes{
				"hx-target":   "#content",
				"hx-push-url": "true",
			}),
			models.WithParams(paramsOptions...),
		)

		categoryFilters = append(categoryFilters, NewCategoryFilter(category.Name, active, route))
	}

	return categoryFilters
}
