// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"github.com/a-h/templ"
	"github.com/joshuar/go-templ-daisyui/actions/button"
	"github.com/joshuar/go-templ-daisyui/display/icon"
	"github.com/joshuar/go-templ-daisyui/modifiers/color"
	"github.com/joshuar/go-templ-daisyui/modifiers/size"

	"github.com/joshuar/go-feed-me/internal/models"
)

// CategoryFilterStatus tracks the filter status of an individual category.
type CategoryFilterStatus struct {
	Active     bool
	Attributes templ.Attributes
}

// CategoryFilterProps tracks status of category filters.
type CategoryFilterProps struct {
	Categories map[models.Category]CategoryFilterStatus
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
