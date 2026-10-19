/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package handlers

import (
	"net/http"

	"github.com/immanent-tech/foragd/web/templates"
)

type About struct {
	*Page
}

func (m *Manager) HandleAbout() http.HandlerFunc {
	about := &About{
		Page: m.NewPage(
			"/about",
			WithSummary("About"),
			WithDescription("Why I built Foragd"),
			WithMetadataDescription(
				"Learn about Foragd, a beautiful, web based, online feed reader. Keep your RSS, Atom and other syndication sources in one place. Stay up to date with news, blogs and other online sources, across your mobile, tablet, desktop and laptop. Understand the design and features of Foragd.",
			),
			WithTemplate(templates.About()),
		),
	}
	return about.Handle()
}
