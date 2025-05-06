// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"context"
	"slices"

	"github.com/a-h/templ"
	"github.com/joshuar/go-templ-daisyui/actions/button"

	"github.com/joshuar/go-feed-me/models"
)

// CategoryFilters is a slice of category filters.
type CategoryFilters []CategoryFilter

// CategoryFilter is a filter for viewing objects by category.
type CategoryFilter struct {
	models.CategoryCount
	attributes templ.Attributes
	active     bool
}

// BuildCategoryFilters generates a list of category filters.
func BuildCategoryFilters(ctx context.Context, categoryCounts models.CategoryCounts) CategoryFilters {
	categoryFilters := make(CategoryFilters, 0, len(categoryCounts))
	for categoryCount := range slices.Values(categoryCounts.GetTopCategories(10)) {
		route := models.NewRouteFromCtx(ctx, models.WithAttributes(viewAttributes))
		var active bool
		if slices.Contains(models.FiltersFromCtx(ctx).Categories, categoryCount.Category) {
			active = true
			route.UnsetCategories()
		} else {
			route.SetCategories(categoryCount.Category)
		}
		categoryFilters = append(categoryFilters, CategoryFilter{
			CategoryCount: categoryCount,
			attributes:    route.GetAttributes(),
			active:        active,
		})
	}

	return categoryFilters
}

// ViewFilters is a slice of filters for changing the view.
type ViewFilters []ViewFilter

// ViewFilter is a filter to apply a certain view.
type ViewFilter struct {
	*button.Props
}

// newViewFilter creates a new view filter.
func newViewFilter(ctx context.Context, view models.View) ViewFilter {
	route := models.NewRouteFromCtx(ctx, models.WithAttributes(viewAttributes))
	route.SetView(view)
	viewFilter := ViewFilter{}
	viewFilter.Props = button.Build(
		button.WithSize(button.SM),
		button.WithColor(button.Accent),
		button.WithContent(string(view)),
		button.WithExtraAttributes(route.GetAttributes()),
		button.WithExtraClasses("join-item"),
	)
	if view == models.FiltersFromCtx(ctx).View {
		button.WithStyle(button.Outline)(viewFilter.Props)
	} else {
		button.WithStyle(button.Dashed)(viewFilter.Props)
	}
	// viewFilter.Props = badge.Build(
	// 	badge.WithSize(badge.SM),
	// 	badge.WithColor(badge.Accent),
	// 	badge.WithContent(string(view)),
	// 	badge.WithExtraAttributes(route.GetAttributes()),
	// )
	// Style based on which view is active.
	// if view == models.FiltersFromCtx(ctx).View {
	// 	badge.WithStyle(badge.Outline)(viewFilter.Props)
	// } else {
	// 	badge.WithStyle(badge.Dashed)(viewFilter.Props)
	// }
	return viewFilter
}

// BuildViewFilters creates all view filters.
func BuildViewFilters(ctx context.Context) ViewFilters {
	return []ViewFilter{
		newViewFilter(ctx, models.ViewRead),
		newViewFilter(ctx, models.ViewUnread),
		newViewFilter(ctx, models.ViewAll),
	}
}
