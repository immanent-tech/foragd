// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"fmt"
	"slices"
	"strings"
)

func (a *Author) String() string {
	if a.Email != nil {
		return fmt.Sprintf("%s (%s)", a.Name, *a.Email)
	}
	return a.Name
}

func generateAuthors(data []string) []Author {
	authors := make([]Author, 0)
	for a := range slices.Values(data) {
		thisAuthor := Author{}
		names := make([]string, 0)
		parts := strings.Split(a, " ")
		for p := range slices.Values(parts) {
			switch {
			case strings.HasPrefix(p, "http"):
				thisAuthor.URL = &p
			case strings.Contains(p, "@"):
				thisAuthor.Email = &p
			default:
				names = append(names, p)
			}
		}
		thisAuthor.Name = strings.Join(names, " ")
		authors = append(authors, thisAuthor)
	}
	return authors
}
