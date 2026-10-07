/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package templates

import (
	"slices"

	"github.com/a-h/templ"

	"github.com/immanent-tech/foragd/models"
)

type ListControls struct {
	Path         string
	Filters      models.ListFilters
	Categories   models.Categories
	Languages    map[string]int64
	ExtraActions []templ.Component
	ExtraButtons []templ.Component
	DisableSort  bool
	DisableView  bool
}

// NewListControls renders an element containing controls (filtering, sorting, view) for a list.
func NewListControls(path string, filters models.ListFilters, options ...ListControlOption) *ListControls {
	controls := &ListControls{
		Path:    path,
		Filters: filters,
	}
	for option := range slices.Values(options) {
		option(controls)
	}
	// Reset any pagination for controls.
	controls.Filters.From = nil
	controls.Filters.UpTo = nil
	controls.Filters.SearchAfter = nil
	return controls
}

type ListControlOption func(*ListControls)

// WithListActions option adds additional actions to the mobile menu.
func WithListActions(actions ...templ.Component) ListControlOption {
	return func(lc *ListControls) {
		lc.ExtraActions = append(lc.ExtraActions, actions...)
	}
}

// WithListButtons option adds additional buttons at the top of the list.
func WithListButtons(buttons ...templ.Component) ListControlOption {
	return func(lc *ListControls) {
		lc.ExtraButtons = append(lc.ExtraButtons, buttons...)
	}
}

// WithListCategories option sets category filters for the list.
func WithListCategories(categories models.Categories) ListControlOption {
	return func(lc *ListControls) {
		if len(categories) > 0 {
			lc.Categories = categories
		}
	}
}

// WithListLanguages option sets language filters for the list.
func WithListLanguages(langs map[string]int64) ListControlOption {
	return func(lc *ListControls) {
		if len(langs) > 0 {
			lc.Languages = langs
		}
	}
}

// WithoutListSorting option will disable showing the sort menu.
func WithoutListSorting() ListControlOption {
	return func(lc *ListControls) {
		lc.DisableSort = true
	}
}

// WithoutListView option will disable showing the view menu.
func WithoutListView() ListControlOption {
	return func(lc *ListControls) {
		lc.DisableView = true
	}
}

type MarkAction struct {
	Path  string
	Label string
	Read  bool // true => "mark read" variant (shows opened-mail icon)
}

func NewMarkAction(path, label string, view models.View) MarkAction {
	if view == models.ViewUnread {
		return MarkAction{Path: path + "/" + string(models.MarkRead), Label: label + " read", Read: true}
	}
	return MarkAction{Path: path + "/" + string(models.MarkUnread), Label: label + " unread"}
}
