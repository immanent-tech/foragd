// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"fmt"
	"net/http"

	feeds "github.com/immanent-tech/go-syndication"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/opengraph"
)

func Viewer() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		title := "Feed Viewer"
		description := "Search for and view syndicated content (RSS, Atom, JSONFeed feeds) on any site."
		switch req.Method {
		case http.MethodGet:
			renderPage(
				templates.NewPage(
					templates.Viewer(),
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
		case http.MethodPost:
			// Get the submitted URL.
			url := req.Form.Get("url")

			// Parse the URL and find feed content.
			feed, err := feeds.NewFeedFromURL(req.Context(), url)
			if err != nil {
				HandleExternalError(&models.APIError{
					InternalError: fmt.Errorf("parse failed"),
					StatusCode:    http.StatusInternalServerError,
				}).ServeHTTP(res, req)
				return
			}

			renderPartial(templates.NewPartial(templates.ViewerResults(feed))).ServeHTTP(res, req)
		}
	}
}
