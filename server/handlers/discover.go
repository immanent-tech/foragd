// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
)

type Discover struct {
	*Page
}

func (m *Manager) HandleDiscover() http.HandlerFunc {
	page := &Discover{
		Page: m.NewPage("/discover",
			WithSummary("Discover feeds"),
			WithDescription("Browse by category or search feeds from the Foragd database"),
			WithNavigation(templates.NavDiscover),
		),
	}
	return func(res http.ResponseWriter, req *http.Request) {
		request, err := parseForm[*models.SuggestFeedsRequest](req)
		if err != nil {
			m.HandleInternalError(http.StatusUnprocessableEntity, err).ServeHTTP(res, req)
			return
		}

		page.template = templates.Discover(request)
		page.Render().ServeHTTP(res, req)
	}
}

func (m *Manager) HandleDiscoverSuggestions(feedSvc FeedService) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		request, err := parseForm[*models.SuggestFeedsRequest](req)
		if err != nil {
			m.HandleInternalError(http.StatusUnprocessableEntity, err).ServeHTTP(res, req)
			return
		}
		results, err := feedSvc.SuggestFeeds(req.Context(), request)
		if err != nil {
			m.HandleInternalError(http.StatusUnprocessableEntity, err).ServeHTTP(res, req)
			return
		}
		m.NewPartial(templates.DiscoverSuggestions(*results)).Render().ServeHTTP(res, req)
	}
}
