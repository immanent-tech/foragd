// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"github.com/a-h/templ"
	components "github.com/joshuar/go-templ-daisyui"
	"github.com/joshuar/go-templ-daisyui/actions/button"
	"github.com/joshuar/go-templ-daisyui/attributes"
	"github.com/joshuar/go-templ-daisyui/display/icon"
	"github.com/joshuar/go-templ-daisyui/modifiers/color"
	"github.com/joshuar/go-templ-daisyui/modifiers/size"

	"github.com/joshuar/go-feed-me/internal/models"
)

// ViewFilterProps tracks the status of the view filter.
type ViewFilterProps struct {
	Active     models.View
	Attributes map[models.View]templ.Attributes
}

func BuildViewFilter(path string, filters *models.APIFilters) *ViewFilterProps {
	return &ViewFilterProps{
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

// CategoryFilterStatus tracks the filter status of an individual category.
type CategoryFilterStatus struct {
	Active     bool
	Attributes templ.Attributes
}

// CategoryFilterProps tracks status of category filters.
type CategoryFilterProps struct {
	Categories map[models.Category]CategoryFilterStatus
}

type SideOption components.Option[*SideProps]

type SideProps struct {
	ActionButtons  []*button.Props
	FilterControls []templ.Component
	*attributes.Attributes
}

func WithActionButtons(buttons ...*button.Props) SideOption {
	return func(p *SideProps) {
		p.ActionButtons = buttons
	}
}

func WithFilterControls(filters ...templ.Component) SideOption {
	return func(p *SideProps) {
		p.FilterControls = filters
	}
}

func WithID(id attributes.ID) SideOption {
	return func(p *SideProps) {
		p.SetID(id)
	}
}

func WithExtraAttributes(attrs templ.Attributes) SideOption {
	return func(p *SideProps) {
		p.AddAttributes(attrs)
	}
}

func BuildSide(options ...SideOption) *SideProps {
	side := &SideProps{
		Attributes: attributes.New(),
	}

	for _, option := range options {
		option(side)
	}

	return side
}

func AddSubscriptionButton(attributes templ.Attributes) *button.Props {
	return button.Build(
		button.WithSize(size.SM),
		button.WithID("add_subscription"),
		button.WithThemeColor(color.Primary, false),
		button.WithContent(icon.Build("fa-plus")),
		button.WithExtraAttributes(attributes),
	)
}

func BuildSideDrawer() templ.Component {
	return BuildSide(
		WithID("drawer_menu"),
		WithActionButtons(
			AddSubscriptionButton(templ.Attributes{
				"hx-get":    "/subscription/new",
				"hx-target": "#command_modal",
				"_":         "on htmx:afterOnLoad wait 10ms then add .modal-open to #add_subscription_modal",
			}),
		),
		WithExtraAttributes(templ.Attributes{
			"hx-target":   "#drawer_menu",
			"hx-swap-oob": "true",
		}),
	).Show()
}
