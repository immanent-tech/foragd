// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/views"
	"github.com/joshuar/go-feed-me/web/templates/layouts"
)

// Index handles displaying the index or front page of the site.
func Index() http.HandlerFunc {
	return alice.New(
		RouteLogger,
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		indexLayout := &layouts.IndexLayout{}
		ctx := templ.WithChildren(req.Context(), indexLayout.FullRender())
		if err := views.NewPage("Go Feed Me").Render(ctx, res); err != nil {
			slogctx.FromCtx(req.Context()).Error("Failed to render page template.", slog.Any("error", err))
			http.Error(res, "Failed to render page content.", http.StatusInternalServerError)
		}
	}).ServeHTTP
}
