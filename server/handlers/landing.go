// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"log/slog"
	"net/http"

	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-feed-me/web/templates"
	"github.com/immanent-tech/go-feed-me/web/templates/layouts"
)

// Landing handles displaying the landing page of the site.
func Landing() http.HandlerFunc {
	template := templates.Page("Go Feed Me", layouts.Landing())
	return alice.New(
		routeLogger,
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		err := template.Render(req.Context(), res)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Failed to render page.", slog.Any("error", err))
			http.Error(res, "Failed to render page template.", http.StatusInternalServerError)
			return
		}
	}).ServeHTTP
}
