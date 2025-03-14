// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"github.com/a-h/templ"

	"github.com/joshuar/go-templ-daisyui/attributes"

	"github.com/joshuar/go-feed-me/internal/api"
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

// buildShowFeedsRoute builds an api.Route for /home/feeds with the given
// filters. This can be used with components that need to create an action for
// /home/feeds.
func buildShowFeedsRoute(filters *api.Filters) *api.Route {
	return api.BuildRoute(api.FeedsRoute,
		api.WithParams(
			api.WithViewParam(filters.GetView()),
			api.WithCountParam(filters.GetCount()),
			api.WithSortParam(filters.GetSort()),
			api.WithCategoriesParam(filters.GetCategories()...),
		),
		api.WithAttributes(commonRouteAttributes),
	)
}

// buildShowItemsRoute builds an api.Route for /home/items with the given
// filters. This can be used with components that need to create an action for
// /home/items.
func buildShowItemsRoute(filters *api.Filters) *api.Route {
	// Build the route.
	return api.BuildRoute(api.ItemsRoute,
		api.WithParams(
			api.WithViewParam(filters.GetView()),
			api.WithCountParam(filters.GetCount()),
			api.WithFeedsParam(filters.GetFeeds()...),
			api.WithSortParam(filters.GetSort()),
			api.WithCategoriesParam(filters.GetCategories()...),
		),
		api.WithAttributes(commonRouteAttributes),
	)
}

// buildHomeRoute will build the appropriate /home route for the given path
// string and filters.
func buildHomeRoute(path string, filters *api.Filters) *api.Route {
	switch path {
	case api.FeedsRoute:
		return buildShowFeedsRoute(filters)
	case api.ItemsRoute:
		return buildShowItemsRoute(filters)
	}

	return nil
}
