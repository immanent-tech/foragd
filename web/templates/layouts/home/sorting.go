// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"github.com/a-h/templ"

	"github.com/joshuar/go-templ-daisyui/navigation/link"

	"github.com/joshuar/go-feed-me/internal/models"
)

func feedsSortOptions(path string, filters models.Filters) []templ.Component {
	var sorts []templ.Component

	// Add sorting options for updated date.
	sorts = append(sorts,
		sortLink("Updated: Newest->Oldest", models.SortLastUpdatedDesc, path, filters),
		sortLink("Updated: Oldest->Newest", models.SortLastUpdatedAsc, path, filters),
	)
	// If not viewing read items, add additional sorting options on unread count.
	if !filters.ViewRead() {
		sorts = append(sorts,
			sortLink("Unread Count: Desc", models.SortUnreadCountDesc, path, filters),
			sortLink("Unread Count: Asc", models.SortUnreadCountAsc, path, filters),
		)
	}

	return sorts
}

func itemsSortOptions(path string, filters models.Filters) []templ.Component {
	var sorts []templ.Component

	// Add sorting options for updated date.
	sorts = append(sorts,
		sortLink("Updated: Newest->Oldest", models.SortLastUpdatedDesc, path, filters),
		sortLink("Updated: Oldest->Newest", models.SortLastUpdatedAsc, path, filters),
	)

	return sorts
}

func sortLink(text string, sort models.Sort, path string, filters models.Filters) templ.Component {
	action := buildHomeAction(path, filters)
	action.AddAttribute(models.ParamSortBy, string(sort.SortBy))
	action.AddAttribute(models.ParamSortOrder, string(sort.SortOrder))

	return link.Build(
		link.WithContent(text),
		link.WithExtraAttributes(action.Attributes()),
		link.WithUnderlineOnHover(),
	).Show()
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
