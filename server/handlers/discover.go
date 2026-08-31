// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/a-h/templ"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/service"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/element"
)

type Discover struct {
	title    templates.PageTitle
	template templ.Component
}

func (h *Discover) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(h.template,
			templates.WithPageTitle(h.title),
		)).ServeHTTP(res, req)
}

func (h *Discover) PartialResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(h.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle(h.title)).ServeHTTP(res, req)
	templ.Handler(templates.SideBar(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
	templ.Handler(templates.Dock(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
}

func HandleDiscover() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		request, err := parseForm[*models.SuggestFeedsRequest](req)
		if err != nil {
			HandleInternalError(http.StatusUnprocessableEntity, err).ServeHTTP(res, req)
			return
		}

		RenderInternalPage(
			&AddSubscription{
				title: templates.PageTitle{
					Summary:     "Discover Feeds",
					Description: "Discover Feeds from the Foragd database",
				},
				template: templates.Discover(request),
			},
		).ServeHTTP(res, req)
	}
}

func HandleDiscoverSuggestions() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		request, err := parseForm[*models.SuggestFeedsRequest](req)
		if err != nil {
			HandleInternalError(http.StatusUnprocessableEntity, err).ServeHTTP(res, req)
			return
		}
		results, err := service.SuggestFeeds(req.Context(), request)
		if err != nil {
			HandleInternalError(http.StatusUnprocessableEntity, err).ServeHTTP(res, req)
			return
		}
		RenderPartial(&PartialTemplate{
			template: templates.DiscoverSuggestions(*results),
		}).ServeHTTP(res, req)
	}
}
