// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/indaco/teseo/schemaorg"

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

type Landing struct {
	template templ.Component
}

func (m *Manager) HandleLanding() http.HandlerFunc {
	metadata := &pageMetadata{
		Title: templates.PageTitle{
			Summary:     "RSS and Atom Feed Reader",
			Description: "View RSS, Atom and other syndicated content in your browser",
		},
		Description: "Foragd is a beautiful, web based, online feed reader. Keep your RSS, Atom and other syndication sources in one place.",
		Path:        "/",
		ImagePath:   "/content/logo-vertical-light.webp",
		baseURL:     m.AppConfig.GetBaseURL(),
	}
	return func(res http.ResponseWriter, req *http.Request) {
		RenderExternalPage(&Landing{
			template: templates.CreatePage(templates.Landing(),
				templates.WithPageTitle(metadata.Title),
				templates.WithOpenGraphMetadata(metadata.OpengraphData()),
				templates.WithJSONLDSchema(
					generateSiteJSONLD(metadata.baseURL),
					orgJsonLd,
				),
			),
		}).ServeHTTP(res, req)
	}
}

func (p *Landing) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(p.template).ServeHTTP(res, req)
}
