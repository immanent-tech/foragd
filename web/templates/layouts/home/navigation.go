// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/web/templates/action"
)

type Navigation struct {
	Path    string
	Filters *models.Filters
}

func (r Navigation) AsAction() *action.Action {
	if r.Filters != nil {
		return action.Build(r.Path,
			action.WithParams(r.Filters.ToQueryParams()),
		)
	}
	return action.Build(r.Path)
}

func NewNavigation(path string, filters *models.Filters) Navigation {
	return Navigation{
		Path:    path,
		Filters: filters,
	}
}
