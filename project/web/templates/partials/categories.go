// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package partials

import (
	"github.com/a-h/templ"

	"github.com/joshuar/go-feed-me/internal/models"
)

type CategoryFilter struct {
	attributes templ.Attributes
	active     bool
}

type CategoryFilters map[models.Category]CategoryFilter

func (f CategoryFilters) Add(category models.Category, active bool, attributes templ.Attributes) {
	if _, found := f[category]; !found {
		f[category] = CategoryFilter{attributes: attributes, active: active}
	}
}
