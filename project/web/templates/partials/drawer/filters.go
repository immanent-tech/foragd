// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package drawer

import (
	"github.com/a-h/templ"

	"github.com/joshuar/go-feed-me/internal/models"
)

// ViewFilterProps tracks the status of the view filter.
type ViewFilterProps struct {
	Active     models.View
	Attributes map[models.View]templ.Attributes
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
