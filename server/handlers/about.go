// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/a-h/templ"

	"github.com/immanent-tech/foragd/web/templates"
)

type About struct {
	template templ.Component
}

func (p *About) FullResponse(w http.ResponseWriter, r *http.Request) {
	templ.Handler(p.template).ServeHTTP(w, r)
}

func (m *Manager) HandleAbout() http.HandlerFunc {
	metadata := &pageMetadata{
		Title: templates.PageTitle{
			Summary:     "About",
			Description: "Why I built Foragd",
		},
		Description: "Learn about Foragd, a beautiful, web based, online feed reader. Keep your RSS, Atom and other syndication sources in one place. Stay up to date with news, blogs and other online sources, across your mobile, tablet, desktop and laptop. Understand the design and features of Foragd.",
		Path:        "/about",
		ImagePath:   "/content/logo-vertical-light.webp",
		baseURL:     m.AppConfig.GetBaseURL(),
	}
	return func(res http.ResponseWriter, req *http.Request) {
		RenderExternalPage(&About{
			template: templates.CreatePage(m.AppConfig, m.SessionMgr, templates.About(),
				templates.WithPageTitle(metadata.Title),
				templates.WithPageDescription(metadata.Description),
				templates.WithOpenGraphMetadata(metadata.OpengraphData()),
				templates.WithJSONLDSchema(
					generateSiteJSONLD(m.AppConfig.GetBaseURL()),
					metadata.JSONLD(),
				),
			),
		}).ServeHTTP(res, req)
	}
}
