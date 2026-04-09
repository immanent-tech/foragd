// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/immanent-tech/foragd/web/templates"
)

type ComparisonPage struct{}

func (p *ComparisonPage) FullResponse(res http.ResponseWriter, req *http.Request) {
	caser := cases.Title(language.English)
	service := caser.String(chi.RouteContext(req.Context()).URLParam("service"))
	templ.Handler(
		templates.CreatePage(templates.Comparison(service),
			templates.WithPageTitle("Compare Foragd vs "+service),
		),
	).ServeHTTP(res, req)
}

func HandleComparison() http.HandlerFunc {
	return RenderExternalPage(&ComparisonPage{})
}
