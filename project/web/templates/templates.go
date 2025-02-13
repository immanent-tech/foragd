// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package templates

type (
	Option[T any]   func(T)
	ComponentOption Option[*Component]
	ActionOption    Option[*Action]
)
