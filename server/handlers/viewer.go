/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package handlers

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/validation"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
)

type Viewer struct {
	*Page
}

// HandleViewer handles powering the feed viewer page.
func (m *Manager) HandleViewer(feedSvc FeedService) http.HandlerFunc {
	page := &Viewer{
		Page: m.NewPage(
			"/viewer",
			WithSummary("Free RSS, Atom and JSONFeed Viewer"),
			WithDescription("Preview any website's feed"),
			WithMetadataDescription(
				"Foragd's free feed viewer instantly shows RSS, Atom, and JSONFeed content for any website. Paste a URL and preview syndicated posts. No account required.",
			),
		),
	}
	return func(res http.ResponseWriter, req *http.Request) {
		switch fetchErr := models.NewErrorMessage(
			"Unable to find feed at provided URL",
			"No feed details could be fetched from the given URL. This could be a temporary error.",
		); req.Method {
		case http.MethodGet:
			if !strings.HasPrefix(req.URL.Path, "/viewer/url") {
				page.template = templates.Viewer(nil, nil)
				page.Handle().ServeHTTP(res, req)
				return
			}
			rawURL, err := validation.SanitizeURLString(chi.URLParam(req, "*"))
			if err != nil {
				slogctx.FromCtx(req.Context()).Error("Could not fetch feed details.",
					slog.Any("error", err),
				)
				page.template = templates.Viewer(nil, fetchErr)
				page.Handle().ServeHTTP(res, req)
				return
			}
			feedURL, err := models.NormalizeFeedURL(rawURL)
			if err != nil {
				slogctx.FromCtx(req.Context()).Error("Could not fetch feed details.",
					slog.Any("error", err),
				)
				page.template = templates.Viewer(nil, fetchErr)
				page.Handle().ServeHTTP(res, req)
				return
			}

			// Parse the URL and find feed content.
			feed, err := feedSvc.FetchFeed(req.Context(), feedURL.String())
			if err != nil {
				slogctx.FromCtx(req.Context()).Error("Could not fetch feed details.",
					slog.Any("error", err),
				)
				page.template = templates.Viewer(nil, fetchErr)
				page.Handle().ServeHTTP(res, req)
				return
			}
			page.template = templates.Viewer(feed, nil)
			page.Handle().ServeHTTP(res, req)
		case http.MethodPost:
			// Parse the URL and find feed content.
			feedURL, err := models.NormalizeFeedURL(req.FormValue("url"))
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Viewer failed to parse feed.",
					slog.Any("error", err),
				)
				m.NewPartial(templates.ViewerError(fetchErr)).Handle().ServeHTTP(res, req)
				return
			}

			feed, err := feedSvc.FetchFeed(req.Context(), feedURL.String())
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Viewer failed to parse feed.",
					slog.Any("error", err),
				)
				m.NewPartial(templates.ViewerError(fetchErr)).Handle().ServeHTTP(res, req)
				return
			}

			m.NewPartial(templates.ViewerResults(feed)).Handle().ServeHTTP(res, req)
		}
	}
}
