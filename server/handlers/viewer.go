// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
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
				renderPartial(
					templates.NewPartial(
						templates.ErrorMessage(
							models.NewErrorMessage(
								"Unable inspect URL",
								"This might be a temporary issue, please try again",
							),
						),
					)).ServeHTTP(res, req)
				return
			}

			renderPartial(templates.NewPartial(templates.ViewerResults(feed))).ServeHTTP(res, req)
		}
	}
}
