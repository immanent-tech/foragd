// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/a-h/templ"
	feeds "github.com/immanent-tech/go-syndication"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/opengraph"
)

// Viewer holds data for the feed viewer page.
type Viewer struct {
	feed *feeds.Feed
}

// FullResponse renders the full viewer page.
func (p *Viewer) FullResponse(res http.ResponseWriter, req *http.Request) {
	title := "Feed Viewer"
	description := "Search for and view syndicated content (RSS, Atom, JSONFeed feeds) on any site."
	templ.Handler(
		templates.CreatePage(
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
}

// PartialResponse returns no content for the viewer as partials are unsupported.
func (p *Viewer) PartialResponse(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusNoContent)
}

// ViewerError holds an error when the viewer failed to parse a url.
type ViewerError struct {
	msg *models.UserMessage
}

// PartialResponse renders an error as the viewer response.
func (p *ViewerError) PartialResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(templates.ViewerError(p.msg)).ServeHTTP(res, req)
}

// HandlViewer handles powering the feed viewer page.
func HandleViewer() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			RenderPage(&Viewer{}).ServeHTTP(res, req)
		case http.MethodPost:
			// Get the submitted URL.
			url := req.FormValue("url")
			// Parse the URL and find feed content.
			feed, err := feeds.NewFeedFromURL(req.Context(), url)
			if err != nil {
				RenderPartial(&ViewerError{
					msg: models.NewErrorMessage("Failed to parse as feed", ""),
				}).ServeHTTP(res, req)
				return
			}

			RenderPartial(&Viewer{feed: feed}).ServeHTTP(res, req)
		}
	}
}
