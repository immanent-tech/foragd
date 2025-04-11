// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"log/slog"
	"net/http"

	"github.com/angelofallars/htmx-go"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/web/templates/layouts"
)

// Index handler handles the index page.
func (s Server) Index(res http.ResponseWriter, req *http.Request) {
	indexPage := layouts.BuildPage(
		layouts.WithHeadOptions("Go Feed Me",
			layouts.WithPageDescription("Welcome to Go Feed Me."),
			layouts.WithPageKeywords("feeds", "atom", "jsonfeed", "rss", "feed reader", "news", "current affairs"),
		),
		layouts.WithPageContent(layouts.IndexLayout()),
	)

	// Render index page template.
	if err := htmx.NewResponse().RenderTempl(req.Context(), res, indexPage.Show()); err != nil {
		slogctx.FromCtx(req.Context()).Error("IndexViewHandler: cannot render template.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}
}
