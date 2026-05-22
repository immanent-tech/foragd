// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/a-h/templ"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
)

type NotFoundPage struct {
	template templ.Component
}

// HandleNotFound handles showing a page for a 404 response.
func HandleNotFound() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		user := models.UserFromCtx(req.Context())
		var layout templ.Component
		if user == nil {
			layout = templates.LayoutExternal(templates.NotFound())
		} else {
			layout = templates.LayoutInternal(
				&templates.InternalLayoutProps{User: user},
				templates.NotFound(),
			)
		}
		RenderInternalPage(&NotFoundPage{template: layout}).ServeHTTP(res, req)
	}
}

func (p *NotFoundPage) FullResponse(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	templ.Handler(templates.CreatePage(p.template)).ServeHTTP(w, r)
}

func (p *NotFoundPage) PartialResponse(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	templ.Handler(templates.CreatePage(p.template), templ.WithFragments(templates.ErrorFragment)).
		ServeHTTP(w, r)
}
