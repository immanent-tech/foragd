// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/a-h/templ"

	"github.com/immanent-tech/foragd/web/templates"
)

type NotFoundPage struct{}

// NotFound handles showing a page for a 404 response.
func HandleNotFound() http.HandlerFunc {
	return RenderPage(&NotFoundPage{})
}

func (p *NotFoundPage) FullResponse(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	templ.Handler(templates.CreatePage(templates.NotFound())).ServeHTTP(w, r)
}

func (p *NotFoundPage) PartialResponse(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	templ.Handler(templates.CreatePage(templates.NotFound()), templ.WithFragments(templates.BodyFragment)).
		ServeHTTP(w, r)
}
