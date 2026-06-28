// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/immanent-tech/go-syndication/opengraph"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/slots"
)

type ComparisonPage struct{}

func (p *ComparisonPage) FullResponse(res http.ResponseWriter, req *http.Request) {
	// Format the service to compare.
	caser := cases.Title(language.English)
	service := caser.String(chi.RouteContext(req.Context()).URLParam("service"))
	// Generate a page title and description.
	title := templates.PageTitle{
		Summary:     "Foragd vs " + service,
		Description: "RSS Feed Reader Comparison",
		Date:        time.Now().Format("2006"),
	}
	description := "A detailed comparison of Foragd and " + service + " covering pricing, features, and which is best for different use cases."

	// Add appropriate additional header metadata.
	ctx := req.Context()
	switch service {
	case "Feedly":
		ctx = slots.WithSlot(ctx, slots.Header, templates.VsFeedlyMeta())
	case "Inoreader", "Innoreader":
		ctx = slots.WithSlot(ctx, slots.Header, templates.VsInoreaderMeta())
	}

	// Render appropriate content.
	templ.Handler(
		templates.CreatePage(templates.Comparison(service),
			templates.WithPageTitle(title),
			templates.WithPageDescription(description),
			templates.WithCanonicalLink(config.GetBaseURL()+req.URL.String()),
			templates.WithOpenGraphMetadata(opengraph.New(
				title.String(),
				"article",
				config.GetBaseURL()+req.URL.String(),
				config.GetBaseURL()+"/content/logo-vertical-light.webp",
				opengraph.WithDescription(description),
			)),
		),
	).ServeHTTP(res, req.WithContext(ctx))
}

func HandleComparison() http.HandlerFunc {
	return RenderExternalPage(&ComparisonPage{})
}
