// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/a-h/templ"

	"github.com/immanent-tech/foragd/web/templates"
)

type Features struct {
	template templ.Component
}

func HandleFeatures() http.HandlerFunc {
	return RenderExternalPage(&Features{
		template: templates.CreatePage(templates.Features(),
			templates.WithPageTitle("A beautiful web-based feed reader"),
		),
	})
}

func (p *Features) FullResponse(w http.ResponseWriter, r *http.Request) {
	templ.Handler(p.template).ServeHTTP(w, r)
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
