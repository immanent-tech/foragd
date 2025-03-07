// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"github.com/a-h/templ"

	"github.com/joshuar/go-templ-daisyui/navigation/link"

	"github.com/joshuar/go-feed-me/internal/api"
)

func BuildFeedsSorting(filters *api.Filters) []templ.Component {
	return []templ.Component{
		sortLink("Updated: Newest->Oldest", api.SortFeedsLastUpdatedDesc, filters),
		sortLink("Updated: Oldest->Newest", api.SortFeedsLastUpdatedAsc, filters),
		sortLink("Unread Count: Desc", api.SortFeedsUnreadCountDesc, filters),
		sortLink("Unread Count: Asc", api.SortFeedsUnreadCountAsc, filters),
	}
}

func sortLink(text string, sort api.Sort, filters *api.Filters) templ.Component {
	route := buildShowFeedsRoute(filters)
	route.SetSort(sort)
	return link.Build(
		link.WithContent(text),
		link.WithExtraAttributes(route.Attributes()),
		link.WithUnderlineOnHover(),
	).Show()
}
