// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/a-h/templ"

	"github.com/immanent-tech/go-syndication/opengraph"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/slots"
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
				config.GetBaseURL(),
				config.GetBaseURL()+"/content/logo-color.webp",
				opengraph.WithDescription(config.AppDescription),
				opengraph.WithSiteName(config.AppName),
			)),
		),
	})
}

func (p *Landing) FullResponse(res http.ResponseWriter, req *http.Request) {
	ctx := slots.WithSlot(req.Context(), slots.Header, templates.LandingHeaderSlot())
	templ.Handler(p.template).ServeHTTP(res, req.WithContext(ctx))
}
