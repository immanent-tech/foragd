// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/indaco/teseo/opengraph"
	"github.com/indaco/teseo/schemaorg"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/web/templates"
)

var sameAs = []string{
	"https://github.com/immanent-tech",
}

var orgJsonLd = schemaorg.NewOrganization(
	"Immanent Tech",
	"https://immanent.tech",
	"https://immanent.tech/content/immanent-tech-icon-dark.svg",
	nil,
	sameAs,
)

// JSON-LD schema for the landing page.
var websiteJsonLd = schemaorg.NewWebSite(
	config.GetBaseURL(),
	config.AppName,
	config.AppName+" RSS and Atom Feed Reader",
	config.AppDescription,
	nil,
)

// Opengraph schema for the landing page.
var websiteOg = opengraph.NewWebSite(
	config.AppName,
	config.GetBaseURL(),
	config.AppDescription,
	config.GetBaseURL()+"/content/logo-vertical-light.webp",
)

type Landing struct {
	template templ.Component
}

func HandleLanding() http.HandlerFunc {
	title := templates.PageTitle{
		Summary:     "RSS and Atom Feed Reader",
		Description: "View RSS, Atom and other syndicated content in your browser",
	}
	return RenderExternalPage(&Landing{
		template: templates.CreatePage(templates.Landing(),
			templates.WithPageTitle(title),
			templates.WithOpenGraphMetadata(websiteOg),
			templates.WithJSONLDSchema(
				websiteJsonLd,
				orgJsonLd,
			),
		),
	})
}

func (p *Landing) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(p.template).ServeHTTP(res, req)
}
