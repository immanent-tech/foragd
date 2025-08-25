// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/justinas/alice"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/web/templates"
	"github.com/joshuar/go-feed-me/web/templates/layouts"
)

// Landing handles displaying the landing page of the site.
func Landing() http.HandlerFunc {
	template := templates.Page("Go Feed Me", layouts.Landing())
	return alice.New(
		RouteLogger,
	).Then(RenderResponse(models.NewResponse(
		models.WithResponseTemplate(template),
	))).ServeHTTP
}
