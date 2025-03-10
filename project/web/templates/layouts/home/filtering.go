// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"slices"

	"github.com/joshuar/go-templ-daisyui/display/badge"
	"github.com/joshuar/go-templ-daisyui/modifiers/color"
	"github.com/joshuar/go-templ-daisyui/modifiers/size"

	"github.com/joshuar/go-feed-me/internal/api"
	"github.com/joshuar/go-feed-me/internal/models"
)

type CategoryFilters []CategoryFilter

type CategoryFilter struct {
	name   models.Category
	route  *api.Route
	active bool
}

func BuildCategoryFilters(filters *api.Filters, categoryCounts []api.CategoryCount, path string) CategoryFilters {
	categoryFilters := make([]CategoryFilter, 0, len(categoryCounts))

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

		categoryFilters = append(categoryFilters,
			CategoryFilter{name: category.Category, active: active, route: route})
	}

	return categoryFilters
}

type ViewFilters struct {
	badges []*badge.Props
}

// viewFilterBadge generates a badge for a view filter.
func viewFilterBadge(view api.View, filters *api.Filters, path string) *badge.Props {
	var route *api.Route
	switch path {
	case "/home/feeds":
		route = buildShowFeedsRoute(filters)
	case "/home/items":
		route = buildShowItemsRoute(filters)
	}
	route.SetView(view)

	// Create the badge component.
	viewBadge := badge.Build(
		badge.WithSize(size.SM),
		badge.WithThemeColor(color.Neutral),
		badge.WithContent(string(view)),
		badge.WithExtraAttributes(route.Attributes()),
	)
	// Style based on which view is active.
	if view == filters.GetView() {
		badge.WithStyle(badge.Outline)(viewBadge)
	} else {
		badge.WithStyle(badge.DashedOutline)(viewBadge)
	}

	return viewBadge
}

func BuildViewFilters(filters *api.Filters, path string) *ViewFilters {
	return &ViewFilters{
		badges: []*badge.Props{
			viewFilterBadge(api.ViewRead, filters, path),
			viewFilterBadge(api.ViewUnread, filters, path),
			viewFilterBadge(api.ViewAll, filters, path),
		},
	}
}
