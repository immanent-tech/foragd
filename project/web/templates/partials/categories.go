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

func NewCategoryFilter(name models.Category, active bool, req string) FeedCategoryFilter {
	route := models.BuildRoute(req,
		models.WithAttributes(templ.Attributes{
			"hx-target":   "#content",
			"hx-push-url": "true",
		}),
	)

	if active {
		route = route.UnsetCategories()
	} else {
		route = route.SetCategories(name)
	}

	return FeedCategoryFilter{
		name:   name,
		active: active,
		route:  route,
	}
}

func BuildCategoryFilters(filteredCategories *[]models.Category, allCategories []models.CategoryCount, path string) []FeedCategoryFilter {
	// Build category filters from the categories.
	filters := make([]FeedCategoryFilter, 0, len(allCategories))

	for _, category := range allCategories {
		var active bool

		if filteredCategories != nil {
			if slices.Contains(*filteredCategories, category.Name) {
				active = true
			}
		}

		filters = append(filters, NewCategoryFilter(category.Name, active, path))
	}

	return filters
}
