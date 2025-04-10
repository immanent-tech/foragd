// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"context"
	"log/slog"
	"slices"

	"github.com/go-chi/chi/v5"
	"github.com/joshuar/go-templ-daisyui/display/badge"
	"github.com/joshuar/go-templ-daisyui/modifiers/color"
	"github.com/joshuar/go-templ-daisyui/modifiers/size"

	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/web/templates"
)

// CategoryFilters is a slice of category filters.
type CategoryFilters []CategoryFilter

// CategoryFilter is a filter for viewing objects by category.
type CategoryFilter struct {
	models.CategoryCount
	action *templates.Action
	active bool
}

// BuildCategoryFilters generates a list of category filters.
func BuildCategoryFilters(ctx context.Context, categoryCounts models.CategoryCounts) CategoryFilters {
	path := chi.RouteContext(ctx).RoutePattern()
	currentFilters := models.FiltersFromCtx(ctx)
	categoryFilters := make(CategoryFilters, 0, len(categoryCounts))
	for categoryCount := range slices.Values(categoryCounts.GetTopCategories(10)) {
		filters := models.NewFilters(
			models.WithCountFilter(currentFilters.Count),
			models.WithViewFilter(currentFilters.View),
			models.WithSortFilters(currentFilters.Sort()),
		)
		var active bool
		if slices.Contains(currentFilters.Categories, categoryCount.Category) {
			active = true
			slog.Info("is active", slog.String("category", categoryCount.Category))
		} else {
			filters.Categories = append(filters.Categories, categoryCount.Category)
		}
		action := templates.BuildAction(path,
			templates.WithQueryParams(filters.ToQueryParams()),
			templates.WithAttributes(viewAttributes),
		)
		categoryFilters = append(categoryFilters, CategoryFilter{
			CategoryCount: categoryCount,
			action:        action,
			active:        active,
		})
	}

	return categoryFilters
}

// ViewFilters is a slice of filters for changing the view.
type ViewFilters []ViewFilter

// ViewFilter is a filter to apply a certain view.
type ViewFilter struct {
	*badge.Props
}

// newViewFilter creates a new view filter.
func newViewFilter(ctx context.Context, view models.View) ViewFilter {
	filters := models.FiltersFromCtx(ctx)
	path := chi.RouteContext(ctx).RoutePattern()
	viewFilter := ViewFilter{}
	viewFilter.Props = badge.Build(
		badge.WithSize(size.SM),
		badge.WithThemeColor(color.Neutral),
		badge.WithContent(string(view)),
		badge.WithExtraAttributes(
			templates.BuildAction(path,
				templates.WithQueryParams(filters.ToQueryParams()),
				templates.WithAttributes(viewAttributes),
				templates.WithQueryParam(models.ParamView, string(view)),
			).Attributes(),
		),
	)
	// Style based on which view is active.
	if view == filters.View {
		badge.WithStyle(badge.Outline)(viewFilter.Props)
	} else {
		badge.WithStyle(badge.DashedOutline)(viewFilter.Props)
	}
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
