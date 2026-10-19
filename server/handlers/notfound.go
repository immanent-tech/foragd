// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
)

type NotFoundPage struct {
	*Page
}

// HandleNotFound handles showing a page for a 404 response.
func (m *Manager) HandleNotFound() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		page := &NotFoundPage{
			Page: m.NewPage(req.URL.Path,
				WithSummary("Not Found"),
				WithDescription("Could not find the requested page"),
			),
		}
		user := models.UserFromCtx(req.Context())
		if user == nil {
			page.template = templates.LayoutExternal(templates.NotFound())
		} else {
			page.template = templates.LayoutInternal(
				"",
				&templates.InternalLayoutProps{User: user},
				templates.NotFound(),
			)
		}
		res.Header().Set("Cache-Control", "no-store")
		res.WriteHeader(http.StatusNotFound)
		page.Render().ServeHTTP(res, req)
	}
}
