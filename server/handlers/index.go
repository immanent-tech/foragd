// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/justinas/alice"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/web/templates/pages"
)

// Index handles displaying the index or front page of the site.
func Index() http.HandlerFunc {
	page := &pages.Index{}
	return alice.New(
		RouteLogger,
	).Then(RenderResponse(models.NewResponse(
		models.WithResponseTemplate(page.Template()),
	))).ServeHTTP
}
