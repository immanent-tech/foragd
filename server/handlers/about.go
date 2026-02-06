// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/a-h/templ"

	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/helpers/opengraph"
)

type About struct {
	template templ.Component
}

func (p *About) FullResponse(w http.ResponseWriter, r *http.Request) {
	templ.Handler(p.template).ServeHTTP(w, r)
}

func HandleAbout() http.HandlerFunc {
	title := "About | Foragd"
	description := "Learn about Foragd, a beautiful, web based, online feed reader. Keep your RSS, Atom and other syndication sources in one place. Stay up to date with news, blogs and other online sources, across your mobile, tablet, desktop and laptop. Understand the design and features of Foragd."
	return RenderExternalPage(&About{
		template: templates.CreatePage(templates.About(),
			templates.WithPageTitle(title),
			templates.WithPageDescription(description),
			templates.WithOGMetadata(
				opengraph.NewMetadata(
					opengraph.WithTitle(title),
					opengraph.WithDescription(description),
				),
			),
		),
	})
}
