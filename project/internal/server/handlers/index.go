// Copyright (C) 2024 Joshua Rich <joshua.rich@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package handlers

import (
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/web/templates"
	"github.com/joshuar/go-feed-me/web/templates/meta"
)

func Index(res http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		// If not, return HTTP 404 error.
		logging.LogReq(req, http.StatusNotFound).Error("IndexViewHandler: invalid request.")
		http.NotFound(res, req)
	}

	// Define template layout for index page.
	indexTemplate := templates.PageTempl(
		templates.Page{
			Title: "Go Feed Me",
			CustomHeaders: []templ.Component{
				meta.Tag("keywords", "feeds, atom, rss, feed reader"),
				meta.Tag("description", "Welcome to Go Feed Me."),
			},
		},
		templates.IndexPage(),
	)

	// Render index page template.
	if err := htmx.NewResponse().RenderTempl(req.Context(), res, indexTemplate); err != nil {
		logging.LogReq(req, http.StatusInternalServerError).Error("IndexViewHandler: cannot render template.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)
		slog.Info("here")
		return
	}
}
