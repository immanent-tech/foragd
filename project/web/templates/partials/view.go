// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package partials

import (
	"github.com/a-h/templ"

	"github.com/joshuar/go-templ-daisyui/display/badge"
	"github.com/joshuar/go-templ-daisyui/modifiers/color"
	"github.com/joshuar/go-templ-daisyui/modifiers/size"

	"github.com/joshuar/go-feed-me/internal/api"
)

type ViewFilter struct {
	routes []*badge.Props
}

func BuildViewFilter(filters *api.APIFilters, path string) *ViewFilter {
	return &ViewFilter{
		routes: []*badge.Props{
			newViewFilter(api.ViewRead, filters, path),
			newViewFilter(api.ViewUnread, filters, path),
			newViewFilter(api.ViewAll, filters, path),
		},
	}
}

// newViewFilter generates a badge for a view filter.
func newViewFilter(view api.View, filters *api.APIFilters, path string) *badge.Props {
	// Common route attributes.
	attributes := templ.Attributes{
		"hx-target":   "#content",
		"hx-push-url": "true",
		"hx-swap":     "morph:outerHTML",
	}

	var params []api.ParamsOption

	// Common route parameters.
	params = append(params,
		api.WithViewParam(view),
		api.WithCountParam(filters.GetCount()),
	)
	// If the path is for viewing items, add any feeds filters.
	if path == "/home/items" {
		params = append(params, api.WithFeedsParam(filters.GetFeeds()...))
	}
	// Build the route.
	route := api.BuildRoute(path,
		api.WithParams(params...),
		api.WithAttributes(attributes),
	)
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
