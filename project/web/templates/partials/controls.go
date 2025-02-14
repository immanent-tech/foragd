// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package partials

import (
	"github.com/a-h/templ"

	"github.com/joshuar/go-feed-me/internal/models"
)

// StateFilterProps contains data for filtering by state.
type StateFilterProps struct {
	Active     models.View
	Attributes map[models.View]templ.Attributes
}

// StateFilter creates a control for filtering on view state (i.e., read/unread/all).
func StateFilter(path string, filters *models.APIFilters) *StateFilterProps {
	return &StateFilterProps{
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
