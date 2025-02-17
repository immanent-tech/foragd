// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package appbar

import (
	"github.com/joshuar/go-templ-daisyui/modifiers/color"
	"github.com/joshuar/go-templ-daisyui/navigation/navbar"
)

func AppBar() *navbar.Props {
	return navbar.Build(navbar.WithID("content_app_bar"),
		navbar.WithBaseColor(color.Base200),
		navbar.NavBarStart(appBarTopLeft()),
		navbar.NavBarEnd(appBarTopRight()),
		navbar.NavBarCenter(appBarTopCenter()))
}
