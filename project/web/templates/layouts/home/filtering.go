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
	"github.com/joshuar/go-feed-me/web/templates"
)

type CategoryFilters []CategoryFilter

type CategoryFilter struct {
	models.CategoryCount
	action *templates.Action
	active bool
}

func BuildCategoryFilters(filters *api.Filters, categoryCounts models.CategoryCounts, path string) CategoryFilters {
	categoryFilters := make(CategoryFilters, 0, len(categoryCounts))

	for categoryCount := range slices.Values(categoryCounts) {
		action := templates.BuildAction(path,
			templates.WithQueryParams(filters.ToQueryParams()),
			templates.WithAttributes(commonRouteAttributes),
		)
		var active bool
		if slices.Contains(filters.Categories, categoryCount.Category) {
			active = true
			action.RemoveParameter(api.ParamCategories)

		} else {
			action.AddParameter(api.ParamCategories, categoryCount.Category)
		}
		categoryFilters = append(categoryFilters, CategoryFilter{
			CategoryCount: categoryCount,
			action:        action,
			active:        active,
		})
	}

	return categoryFilters
}

type ViewFilters struct {
	badges []*badge.Props
}

// viewFilterBadge generates a badge for a view filter.
func viewFilterBadge(view api.View, filters *api.Filters, path string) *badge.Props {
	action := templates.BuildAction(path,
		templates.WithQueryParams(filters.ToQueryParams()),
		templates.WithAttributes(commonRouteAttributes),
	)
	action.AddParameter(api.ParamView, string(view))

	// Create the badge component.
	viewBadge := badge.Build(
		badge.WithSize(size.SM),
		badge.WithThemeColor(color.Neutral),
		badge.WithContent(string(view)),
		badge.WithExtraAttributes(action.Attributes()),
	)
	// Style based on which view is active.
	if view == filters.View {
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
