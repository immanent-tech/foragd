// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-resty/resty/v2"

	"github.com/immanent-tech/go-base/pkg/htmx"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/element"
)

type Discover struct {
	title    templates.PageTitle
	template templ.Component
	svc      pageServices
}

func (p *Discover) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(
			p.svc.appCfg,
			p.svc.sessionMgr,
			p.template,
			templates.WithPageTitle(p.title),
		)).ServeHTTP(res, req)
}

func (p *Discover) PartialResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(p.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle(p.title)).ServeHTTP(res, req)
	templ.Handler(templates.SideBar(templates.NavDiscover, element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
	templ.Handler(templates.Dock(templates.NavDiscover, element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
}

func (m *Manager) HandleDiscover() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set(htmx.HeaderPushURL, req.URL.String())
		request, err := parseForm[*models.SuggestFeedsRequest](req)
		if err != nil {
			m.HandleInternalError(http.StatusUnprocessableEntity, err).ServeHTTP(res, req)
			return
		}

		RenderInternalPage(
			&Discover{
				title: templates.PageTitle{
					Summary:     "Discover Feeds",
					Description: "Discover Feeds from the Foragd database",
				},
				template: templates.Discover(request),
				svc:      m.NewPageServices(),
			},
		).ServeHTTP(res, req)
	}
}

func (m *Manager) HandleDiscoverSuggestions(feedSvc FeedService, httpClient *resty.Client) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		request, err := parseForm[*models.SuggestFeedsRequest](req)
		if err != nil {
			m.HandleInternalError(http.StatusUnprocessableEntity, err).ServeHTTP(res, req)
			return
		}
		results, err := feedSvc.SuggestFeeds(req.Context(), httpClient, request)
		if err != nil {
			m.HandleInternalError(http.StatusUnprocessableEntity, err).ServeHTTP(res, req)
			return
		}
		RenderPartial(&PartialTemplate{
			template: templates.DiscoverSuggestions(*results),
		}).ServeHTTP(res, req)
	}
}
