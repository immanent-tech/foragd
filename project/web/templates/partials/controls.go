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
func StateFilter(active models.View, urls map[models.View]string) *StateFilterProps {
	return &StateFilterProps{
		Active: active,
		Attributes: map[models.View]templ.Attributes{
			models.ViewRead: {
				"hx-get":      urls[models.ViewRead],
				"hx-target":   "#content",
				"hx-push-url": "true",
				"_":           "on click toggle .checked on #home_drawer_toggle",
			},
			models.ViewUnread: {
				"hx-get":      urls[models.ViewUnread],
				"hx-target":   "#content",
				"hx-push-url": "true",
				"_":           "on click toggle .checked on #home_drawer_toggle",
			},
			models.ViewAll: {
				"hx-get":      urls[models.ViewAll],
				"hx-target":   "#content",
				"hx-push-url": "true",
				"_":           "on click toggle .checked on #home_drawer_toggle",
			},
		},
	}
}
