// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package partials

import (
	"github.com/a-h/templ"

	"github.com/joshuar/go-feed-me/internal/models"
)

type ViewFilter struct {
	Active models.View
	routes map[models.View]templ.Attributes
}

func BuildViewFilter(activeView models.View, req string) *ViewFilter {
	attributes := templ.Attributes{
		"hx-target":   "#content",
		"hx-push-url": "true",
		"hx-swap":     "morph:outerHTML",
	}

	readRoute := models.BuildRoute(req,
		models.WithParams(models.WithViewParam(models.ViewRead)),
		models.WithAttributes(attributes),
	)

	unreadRoute := models.BuildRoute(req,
		models.WithParams(models.WithViewParam(models.ViewUnread)),
		models.WithAttributes(attributes),
	)

	allRoute := models.BuildRoute(req,
		models.WithParams(models.WithViewParam(models.ViewAll)),
		models.WithAttributes(attributes),
	)

	filter := &ViewFilter{
		Active: activeView,
		routes: map[models.View]templ.Attributes{
			models.ViewRead:   readRoute.Attributes(),
			models.ViewUnread: unreadRoute.Attributes(),
			models.ViewAll:    allRoute.Attributes(),
		},
	}

	return filter
}
