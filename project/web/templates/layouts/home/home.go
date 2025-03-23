// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"github.com/a-h/templ"

	"github.com/joshuar/go-templ-daisyui/attributes"

	"github.com/joshuar/go-feed-me/internal/api"
	"github.com/joshuar/go-feed-me/web/templates"
)

// ContentID is the id attribute for the main content area.
var ContentID = attributes.ID("content")

// Common route attributes are common attributes that most actions on /home
// routes will use to update/change content.
var commonRouteAttributes = templ.Attributes{
	"hx-target":   ContentID.Target(),
	"hx-push-url": "true",
	"hx-swap":     "morph:innerHTML",
}

// buildShowFeedsAction builds an api.Route for /home/feeds with the given
// filters. This can be used with components that need to create an action for
// /home/feeds.
func buildShowFeedsAction(filters *api.Filters) *templates.Action {
	return templates.BuildAction(api.FeedsRoute,
		templates.WithQueryParams(filters.ToQueryParams()),
		templates.WithAttributes(commonRouteAttributes),
	)
}

// buildShowItemsAction builds an api.Route for /home/items with the given
// filters. This can be used with components that need to create an action for
// /home/items.
func buildShowItemsAction(filters *api.Filters) *templates.Action {
	// Build the route.
	return templates.BuildAction(api.ItemsRoute,
		templates.WithQueryParams(filters.ToQueryParams()),
		templates.WithAttributes(commonRouteAttributes),
	)
}

// buildHomeAction will build the appropriate /home route for the given path
// string and filters.
func buildHomeAction(path string, filters *api.Filters) *templates.Action {
	switch path {
	case api.FeedsRoute:
		return buildShowFeedsAction(filters)
	case api.ItemsRoute:
		return buildShowItemsAction(filters)
	}

	return nil
}
