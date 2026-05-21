// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/a-h/templ"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/web/templates"
)

type Features struct{}

func HandleFeatures() http.HandlerFunc {
	return RenderExternalPage(&Features{})
}

func (p *Features) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(templates.CreatePage(templates.Features(),
		templates.WithPageTitle("A beautiful web-based feed reader"),
		templates.WithCanonicalLink(config.GetBaseURL()+req.URL.String()),
	)).ServeHTTP(res, req)
}

func HandleFeaturesCollect() http.HandlerFunc {
	return RenderPartial(&PartialTemplate{
		template: templates.FeaturesCollect(),
	})
}

func HandleFeaturesCurate() http.HandlerFunc {
	return RenderPartial(&PartialTemplate{
		template: templates.FeaturesCurate(),
	})
}

func HandleFeaturesConsume() http.HandlerFunc {
	return RenderPartial(&PartialTemplate{
		template: templates.FeaturesConsume(),
	})
}
