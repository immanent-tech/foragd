// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-syndication/opengraph"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
)

type Viewer struct {
	feed   *models.Feed
	errMsg *models.UserMessage
}

// FullResponse renders the full viewer page.
func (p *Viewer) FullResponse(res http.ResponseWriter, req *http.Request) {
	title := "Free RSS Feed Viewer: Preview Any Website's Feed"
	description := "Foragd's free feed viewer instantly shows RSS, Atom, and JSONFeed content for any website. Paste a URL and preview syndicated posts. No account required."
	templ.Handler(
		templates.CreatePage(
			templates.Viewer(p.feed, p.errMsg),
			templates.WithPageTitle(title),
			templates.WithPageDescription(description),
			templates.WithOpenGraphMetadata(opengraph.New(
				title,
				"website",
				os.Getenv("FORAGD_BASEURL")+"/about",
				os.Getenv("FORAGD_BASEURL")+"/content/logo-color.webp",
				opengraph.WithDescription(description),
				opengraph.WithSiteName(config.AppName),
			)),
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
		switch fetchErr := models.NewErrorMessage(
			"Unable to find feed at provided URL",
			"No feed details could be fetched from the given URL. This could be a temporary error.",
		); req.Method {
		case http.MethodGet:
			if !strings.HasPrefix(req.URL.Path, "/viewer/url") {
				RenderExternalPage(&Viewer{}).ServeHTTP(res, req)
				return
			}
			// Parse the URL and find feed content.
			feed, err := models.NewFeedFromURL(req.Context(), chi.URLParam(req, "*"), "", false)
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
			// Parse the URL and find feed content.
			feed, err := models.NewFeedFromURL(req.Context(), req.FormValue("url"), "", false)
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
