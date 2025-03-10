// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"github.com/a-h/templ"

	"github.com/joshuar/go-templ-daisyui/actions/button"
	"github.com/joshuar/go-templ-daisyui/actions/dropdown"
	"github.com/joshuar/go-templ-daisyui/display/icon"
	"github.com/joshuar/go-templ-daisyui/modifiers/color"
	"github.com/joshuar/go-templ-daisyui/modifiers/size"
	"github.com/joshuar/go-templ-daisyui/navigation/link"
	"github.com/joshuar/go-templ-daisyui/navigation/menu"

	"github.com/joshuar/go-feed-me/internal/api"
)

func feedsSortOptions(path string, filters *api.Filters) []templ.Component {
	var sorts []templ.Component

	// Add sorting options for updated date.
	sorts = append(sorts,
		sortLink("Updated: Newest->Oldest", api.SortLastUpdatedDesc, path, filters),
		sortLink("Updated: Oldest->Newest", api.SortLastUpdatedAsc, path, filters),
	)
	// If not viewing read items, add additional sorting options on unread count.
	if !filters.ViewRead() {
		sorts = append(sorts,
			sortLink("Unread Count: Desc", api.SortUnreadCountDesc, path, filters),
			sortLink("Unread Count: Asc", api.SortUnreadCountAsc, path, filters),
		)
	}

	return sorts
}

func itemsSortOptions(path string, filters *api.Filters) []templ.Component {
	var sorts []templ.Component

	// Add sorting options for updated date.
	sorts = append(sorts,
		sortLink("Updated: Newest->Oldest", api.SortLastUpdatedDesc, path, filters),
		sortLink("Updated: Oldest->Newest", api.SortLastUpdatedAsc, path, filters),
	)

	return sorts
}

func sortLink(text string, sort api.Sort, path string, filters *api.Filters) templ.Component {
	var route *api.Route

	switch path {
	case "/home/feeds":
		route = buildShowFeedsRoute(filters)
	case "/home/items":
		route = buildShowItemsRoute(filters)
	}

	route.SetSort(sort)
	return link.Build(
		link.WithContent(text),
		link.WithExtraAttributes(route.Attributes()),
		link.WithUnderlineOnHover(),
	).Show()
}

func BuildSortMenu(path string, filters *api.Filters) templ.Component {
	var sortItems []templ.Component

	switch path {
	case "/home/feeds":
		sortItems = feedsSortOptions(path, filters)
	case "/home/items":
		sortItems = itemsSortOptions(path, filters)
	}

	return dropdown.Build(
		dropdown.WithOpenOptions(
			dropdown.From(dropdown.OpenBottom, true),
		),
		dropdown.WithButton(
			button.WithSize(size.SM),
			button.AsShape(button.Square, false),
			button.WithContent(icon.Build("fa-sort")),
		),
		dropdown.WithMenuContent(
			menu.WithMenuTitle("Sort"),
			menu.WithBaseColor(color.Base200),
			menu.WithItems(sortItems...),
		),
	).Show()
}
