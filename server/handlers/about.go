// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/opengraph"
)

func About() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		title := "About | Foragd"
		description := "Learn about Foragd, a beautiful, web based, online feed reader. Keep your RSS, Atom and other syndication sources in one place. Stay up to date with news, blogs and other online sources, across your mobile, tablet, desktop and laptop. Understand the design and features of Foragd."
		renderPage(
			templates.NewPage(templates.About(),
				templates.WithPageTitle(title),
				templates.WithPageDescription(description),
				templates.WithOGMetadata(
					opengraph.NewMetadata(
						opengraph.WithTitle(title, nil),
						opengraph.WithDescription(description, nil),
					),
				),
			),
		).ServeHTTP(res, req)
	}
}
