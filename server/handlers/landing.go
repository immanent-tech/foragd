// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/a-h/templ"

	"github.com/immanent-tech/foragd/web/templates"
)

type Landing struct {
	template templ.Component
}

func HandleLanding() http.HandlerFunc {
	return RenderExternalPage(&Landing{
		template: templates.CreatePage(templates.Landing()),
	})
}

func (p *Landing) FullResponse(w http.ResponseWriter, r *http.Request) {
	templ.Handler(p.template).ServeHTTP(w, r)
}
