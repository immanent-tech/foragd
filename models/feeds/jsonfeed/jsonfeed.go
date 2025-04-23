// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package jsonfeed contains objects and methods defining the JSONFeed syndication format.
package jsonfeed

func (a *Author) String() string {
	if a.Name != nil {
		return *a.Name
	}
	return ""
}
