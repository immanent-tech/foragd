// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/web/templates"
	"github.com/joshuar/go-feed-me/web/templates/meta"
	"github.com/joshuar/go-feed-me/web/templates/pages"
)

const inputDelay = "delay:500ms"

func Home(res http.ResponseWriter, req *http.Request, cache models.Cache, db models.DB) {
	// Define template layout for index page.
	homePage := templates.PageTempl(
		templates.Page{
			Title: "Home",
			CustomHeaders: []templ.Component{
				meta.Tag("keywords", "gowebly, htmx example page, go with htmx"),
				meta.Tag("description", "Welcome to example! You're here because it worked out."),
			},
		},
		pages.HomePage(),
	)
	// Render index page template.
	if err := htmx.NewResponse().
		RenderTempl(req.Context(), res, homePage); err != nil {
		logging.LogReq(req, http.StatusInternalServerError).Error("IndexViewHandler: cannot render template.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}
}
