// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"github.com/a-h/templ"

	"github.com/joshuar/go-feed-me/internal/models"
)

func feedsSortOptions(path string, filters models.Filters) []templ.Component {
	var sorts []templ.Component

	// Add sorting options for updated date.
	sorts = append(sorts,
		newSortAction(models.SortLastUpdatedDesc, path, filters),
		newSortAction(models.SortLastUpdatedAsc, path, filters),
	)
	// If not viewing read items, add additional sorting options on unread count.
	if !filters.ViewRead() {
		sorts = append(sorts,
			newSortAction(models.SortUnreadCountDesc, path, filters),
			newSortAction(models.SortUnreadCountAsc, path, filters),
		)
	}

	return sorts
}

func itemsSortOptions(path string, filters models.Filters) []templ.Component {
	var sorts []templ.Component

	// Add sorting options for updated date.
	sorts = append(sorts,
		newSortAction(models.SortLastUpdatedDesc, path, filters),
		newSortAction(models.SortLastUpdatedAsc, path, filters),
	)

	return sorts
}

func newSortAction(sort models.Sort, path string, filters models.Filters) templ.Component {
	sortAction := &SortBadge{
		Sort:   sort,
		action: buildHomeAction(path, filters),
	}
	sortAction.action.AddAttribute(models.ParamSortBy, string(sort.SortBy))
	sortAction.action.AddAttribute(models.ParamSortOrder, string(sort.SortOrder))
	return sortAction.Show()
}

func generateSortOptions(filters models.Filters, path string) []templ.Component {
	switch path {
	case models.FeedsRoute:
		return feedsSortOptions(path, filters)
	case models.ItemsRoute:
		return itemsSortOptions(path, filters)
	default:
		return nil
	}
}
