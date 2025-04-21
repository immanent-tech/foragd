// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"context"
	"reflect"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/joshuar/go-feed-me/models"
)

func newSortAction(sort models.Sort, path string, filters models.Filters) templ.Component {
	sortAction := &SortBadge{
		Sort: sort,
	}
	if reflect.DeepEqual(filters.Sort(), sortAction.Sort) {
		sortAction.active = true
	}
	route := models.NewRoute(path, &filters)
	route.SetAttributes(viewAttributes)
	route.SetSortBy(sort.SortBy)
	route.SetSortOrder(sort.SortOrder)
	sortAction.attributes = route.GetAttributes()
	return sortAction.Show()
}

//nolint:contextcheck
func generateSortOptions(ctx context.Context) []templ.Component {
	path := chi.RouteContext(ctx).RoutePattern()
	filters := models.FiltersFromCtx(ctx)

	var sorts []templ.Component
	// Add sorting options for updated date.
	sorts = append(sorts,
		newSortAction(models.SortLastUpdatedDesc, path, filters),
		newSortAction(models.SortLastUpdatedAsc, path, filters),
	)
	// If viewing /home/feeds and not viewing read items, add additional sorting options on unread count.
	if path == models.FeedsRoute && !filters.ViewRead() {
		sorts = append(sorts,
			newSortAction(models.SortUnreadCountDesc, path, filters),
			newSortAction(models.SortUnreadCountAsc, path, filters),
		)
	}

	return sorts
}
