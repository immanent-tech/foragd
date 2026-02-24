// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/helpers/opengraph"
)

type Viewer struct {
	feed   *models.Feed
	errMsg *models.UserMessage
}

// FullResponse renders the full viewer page.
func (p *Viewer) FullResponse(res http.ResponseWriter, req *http.Request) {
	title := "Feed Viewer"
	description := "Search for and view syndicated content (RSS, Atom, JSONFeed feeds) on any site."
	templ.Handler(
		templates.CreatePage(
			templates.Viewer(p.feed, p.errMsg),
			templates.WithPageTitle(title),
			templates.WithPageDescription(description),
			templates.WithOGMetadata(
				opengraph.NewMetadata(
					opengraph.WithTitle(title),
					opengraph.WithDescription(description),
				),
			),
		),
	).ServeHTTP(res, req)
}

type ViewerResponse struct {
	feed *models.Feed
}

// PartialResponse returns no content for the viewer as partials are unsupported.
func (p *ViewerResponse) PartialResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(templates.ViewerResults(p.feed)).ServeHTTP(res, req)
}

// ViewerError holds an error when the viewer failed to parse a url.
type ViewerError struct {
	msg *models.UserMessage
}

// PartialResponse renders an error as the viewer response.
func (p *ViewerError) PartialResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(templates.ViewerError(p.msg)).ServeHTTP(res, req)
}

// HandleViewer handles powering the feed viewer page.
func HandleViewer() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		parseErr := models.NewErrorMessage(
			"Unable to parse provided URL",
			"The URL does not appear to be valid, please check and try again.",
		)
		fetchErr := models.NewErrorMessage(
			"Unable to find feed at provided URL",
			"No feed details could be fetched from the given URL. This could be a temporary error.",
		)

		switch req.Method {
		case http.MethodGet:
			if !strings.HasPrefix(req.URL.Path, "/viewer/url") {
				RenderExternalPage(&Viewer{}).ServeHTTP(res, req)
				return
			}
			feedURL, err := models.FeedURLParser(req.Context(), chi.URLParam(req, "*"))
			if err != nil {
				slogctx.FromCtx(req.Context()).Error("Could not parse URL for viewer.",
					slog.Any("error", err),
				)
				RenderExternalPage(&Viewer{
					errMsg: parseErr,
				}).ServeHTTP(res, req)
				return
			}
			// Parse the URL and find feed content.
			feed, err := models.NewFeedFromURL(req.Context(), feedURL.String(), "", false)
			if err != nil {
				slogctx.FromCtx(req.Context()).Error("Could not fetch feed details.",
					slog.Any("error", err),
				)
				RenderExternalPage(&Viewer{
					errMsg: fetchErr,
				}).ServeHTTP(res, req)
				return
			}

			RenderExternalPage(&Viewer{
				feed: feed,
			}).ServeHTTP(res, req)

		case http.MethodPost:
			// Get the submitted URL.
			feedURL, err := models.FeedURLParser(req.Context(), req.FormValue("url"))
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Viewer failed to parse feed.",
					slog.Any("error", err),
				)
				RenderPartial(&ViewerError{
					msg: parseErr,
				}).ServeHTTP(res, req)
				return
			}

			// Parse the URL and find feed content.
			feed, err := models.NewFeedFromURL(req.Context(), feedURL.String(), "", false)
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Viewer failed to parse feed.",
					slog.Any("error", err),
				)
				RenderPartial(&ViewerError{
					msg: fetchErr,
				}).ServeHTTP(res, req)
				return
			}

			RenderPartial(&ViewerResponse{feed: feed}).ServeHTTP(res, req)
		}
	}
}
