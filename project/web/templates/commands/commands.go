// Copyright (C) 2024 Joshua Rich <joshua.rich@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package commands

import (
	"github.com/a-h/templ"

	components "github.com/joshuar/go-templ-daisyui"
)

var HelpCommand = components.NewButton("Help", "help",
	components.WithModifier(components.ButtonGhost),
	components.WithIcon(components.NewIcon("circle-info"), components.AlignLeft),
	components.WithAttributes[components.Button](templ.Attributes{
		"hx-get": "/home/help",
		"_":      "on htmx:afterOnLoad wait 10ms then add .modal-open to #command-modal",
	}),
)

var SettingsCommand = components.NewButton("Settings", "settings",
	components.WithModifier(components.ButtonGhost),
	components.WithIcon(components.NewIcon("cog"), components.AlignLeft),
	components.WithAttributes[components.Button](templ.Attributes{
		"hx-get":    "/home/settings",
		"hx-target": "#command-modal",
		"_":         "on htmx:afterOnLoad wait 10ms then add .modal-open to #command-modal",
	}),
)

var ProfileCommand = components.NewButton("Profile", "profile",
	components.WithModifier(components.ButtonGhost),
	components.WithIcon(components.NewIcon("user"), components.AlignLeft),
	components.WithAttributes[components.Button](templ.Attributes{
		"hx-get":    "/home/profile",
		"hx-target": "#command-modal",
		"_":         "on htmx:afterOnLoad wait 10ms then add .modal-open to #command-modal",
	}),
)

var AddCommand = components.NewButton("Add", "add",
	components.WithModifier(components.ButtonGhost),
	components.WithIcon(components.NewIcon("plus"), components.AlignLeft),
	components.WithAttributes[components.Button](templ.Attributes{
		"hx-get":    "/home/add",
		"hx-target": "#command-modal",
		"_":         "on htmx:afterOnLoad wait 10ms then add .modal-open to #command-modal",
	}),
)

var PrevCommand = components.NewButton("", "prev",
	components.WithModifier(components.ButtonGhost),
	components.WithIcon(components.NewIcon("arrow-left"), components.AlignLeft),
	components.WithAttributes[components.Button](templ.Attributes{
		"hx-get": "/home/prev",
	}),
)

var NextCommand = components.NewButton("", "next",
	components.WithModifier(components.ButtonGhost),
	components.WithIcon(components.NewIcon("arrow-right"), components.AlignRight),
	components.WithAttributes[components.Button](templ.Attributes{
		"hx-get": "/home/next",
	}),
)
