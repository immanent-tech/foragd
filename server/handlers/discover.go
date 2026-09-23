// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/go-resty/resty/v2"

	"github.com/immanent-tech/go-base/pkg/htmx"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
)

// type Discover struct {
// 	title      templates.PageTitle
// 	template   templ.Component
// 	appCfg     AppConfig
// 	sessionMgr SessionManager
// }

// func (h *Discover) FullResponse(res http.ResponseWriter, req *http.Request) {
// 	templ.Handler(
// 		templates.CreatePage(
// 			h.appCfg,
// 			h.sessionMgr,
// 			h.template,
// 			templates.WithPageTitle(h.title),
// 		)).ServeHTTP(res, req)
// }

// func (h *Discover) PartialResponse(res http.ResponseWriter, req *http.Request) {
// 	templ.Handler(h.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
// 	templ.Handler(templates.UpdateTitle(h.title)).ServeHTTP(res, req)
// 	templ.Handler(templates.SideBar(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
// 	templ.Handler(templates.Dock(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
// }

func (m *Manager) HandleDiscover() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set(htmx.HeaderPushURL, req.URL.String())
		request, err := parseForm[*models.SuggestFeedsRequest](req)
		if err != nil {
			m.HandleInternalError(http.StatusUnprocessableEntity, err).ServeHTTP(res, req)
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
