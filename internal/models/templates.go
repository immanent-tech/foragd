// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import "github.com/a-h/templ"

type Template interface {
	Show(classes ...string) templ.Component
}
