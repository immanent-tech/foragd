// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package state

import (
	"github.com/a-h/templ"

	"github.com/joshuar/go-feed-me/internal/models"
)

// Props contains data for filtering by state.
type Props struct {
	Active     models.View
	Attributes map[models.View]templ.Attributes
}

// BuildFilter creates a state filter from the given data.
func BuildFilter(path string, filters *models.APIFilters) *Props {
	return &Props{
		Active: filters.View,
		Attributes: map[models.View]templ.Attributes{
			models.ViewRead: {
				"hx-get":      filters.BuildURL(path, models.WithView(models.ViewRead)).String(),
				"hx-target":   "#content",
				"hx-push-url": "true",
				"_":           "on click toggle .checked on #home_drawer_toggle",
			},
			models.ViewUnread: {
				"hx-get":      filters.BuildURL(path, models.WithView(models.ViewUnread)).String(),
				"hx-target":   "#content",
				"hx-push-url": "true",
				"_":           "on click toggle .checked on #home_drawer_toggle",
			},
			models.ViewAll: {
				"hx-get":      filters.BuildURL(path, models.WithView(models.ViewAll)).String(),
				"hx-target":   "#content",
				"hx-push-url": "true",
				"_":           "on click toggle .checked on #home_drawer_toggle",
			},
		},
	}
}
