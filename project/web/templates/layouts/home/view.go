// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"github.com/joshuar/go-templ-daisyui/display/badge"
	"github.com/joshuar/go-templ-daisyui/modifiers/color"
	"github.com/joshuar/go-templ-daisyui/modifiers/size"

	"github.com/joshuar/go-feed-me/internal/api"
)

type ViewFilterBadges struct {
	badges []*badge.Props
}

func BuildViewFilter(filters *api.Filters, path string) *ViewFilterBadges {
	return &ViewFilterBadges{
		badges: []*badge.Props{
			viewFilterBadge(api.ViewRead, filters, path),
			viewFilterBadge(api.ViewUnread, filters, path),
			viewFilterBadge(api.ViewAll, filters, path),
		},
	}
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
