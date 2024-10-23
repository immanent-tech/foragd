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

	"github.com/joshuar/go-feed-me/logging"
	"github.com/joshuar/go-feed-me/templates"
	"github.com/joshuar/go-feed-me/templates/meta"
	"github.com/joshuar/go-feed-me/templates/partials"
)

const inputDelay = "delay:500ms"

func Home(res http.ResponseWriter, req *http.Request) {
	// Define template layout for index page.
	homePage := templates.PageTempl(
		templates.Page{
			Title: "Home",
			CustomHeaders: []templ.Component{
				meta.Tag("keywords", "gowebly, htmx example page, go with htmx"),
				meta.Tag("description", "Welcome to example! You're here because it worked out."),
			},
		},
		// partials.TopNavBar(),
		partials.PanedLayout(),
		// partials.ResponsiveLayoutTemplate(),
	)
	// Render index page template.
	if err := htmx.NewResponse().
		AddTrigger(htmx.Trigger("input changed "+inputDelay)).
		AddTrigger(htmx.Trigger("search")).
		RenderTempl(req.Context(), res, homePage); err != nil {
		logging.LogReq(req, http.StatusInternalServerError).Error("IndexViewHandler: cannot render template.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}
}
