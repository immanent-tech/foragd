// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package drawer

import (
	"github.com/a-h/templ"
	"github.com/joshuar/go-templ-daisyui/actions/button"
	"github.com/joshuar/go-templ-daisyui/display/icon"
	"github.com/joshuar/go-templ-daisyui/modifiers/color"
	"github.com/joshuar/go-templ-daisyui/modifiers/size"
)

func AddSubscriptionButton(attributes templ.Attributes) *button.Props {
	return button.Build(
		button.WithSize(size.SM),
		button.WithID("add_subscription"),
		button.WithThemeColor(color.Primary, false),
		button.WithContent(icon.Build("fa-plus")),
		button.WithExtraAttributes(attributes),
	)
}
