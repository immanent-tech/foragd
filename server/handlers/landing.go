// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"
	"os"

	"github.com/a-h/templ"

	"github.com/immanent-tech/go-syndication/opengraph"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/web/templates"
)

type Landing struct {
	template templ.Component
}

func HandleLanding() http.HandlerFunc {
	return RenderExternalPage(&Landing{
		template: templates.CreatePage(templates.Landing(),
			templates.WithPageTitle("A beautiful web-based feed reader"),
			templates.WithOpenGraphMetadata(opengraph.New(
				"Foragd",
				"website",
				os.Getenv("FORAGD_BASEURL"),
				os.Getenv("FORAGD_BASEURL")+"/content/logo-color.webp",
				opengraph.WithDescription(config.AppDescription),
				opengraph.WithSiteName(config.AppName),
			)),
		),
	})
}

func (p *Landing) FullResponse(w http.ResponseWriter, r *http.Request) {
	templ.Handler(p.template).ServeHTTP(w, r)
}
