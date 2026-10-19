// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/go-resty/resty/v2"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/validation"
	"github.com/immanent-tech/go-syndication/linter"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/service"
	"github.com/immanent-tech/foragd/web/templates"
)

type Linter struct {
	*Page
}

func (m *Manager) HandleLinter(httpClient *resty.Client) http.HandlerFunc {
	page := &Linter{
		Page: m.NewPage(
			"/linter",
			WithSummary("Free RSS Feed Linter"),
			WithDescription("Lint Any Website's Feed to check for validation errors and recommendations"),
			WithMetadataDescription(
				"Foragd's free feed RSS linter instantly shows whether a site's feed passes validation and contains recommended values to ensure maximum compatibility",
			),
			WithTemplate(templates.Linter()),
		),
	}
	return func(res http.ResponseWriter, req *http.Request) {
		switch fetchErr := models.NewErrorMessage(
			"Unable to find feed at provided URL",
			"No feed details could be fetched from the given URL. This could be a temporary error.",
		); req.Method {
		case http.MethodGet:
			page.Handle().ServeHTTP(res, req)
		case http.MethodPost:
			rawURL, err := validation.SanitizeURLString(req.FormValue("url"))
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Linter failed to parse feed.",
					slog.String("url", rawURL),
					slog.Any("error", err),
				)
				m.NewPartial(templates.LinterError(fetchErr)).Handle().ServeHTTP(res, req)
				return
			}
			sourceURL, err := url.Parse(rawURL)
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Linter failed to parse feed.",
					slog.String("url", rawURL),
					slog.Any("error", err),
				)
				m.NewPartial(templates.LinterError(fetchErr)).Handle().ServeHTTP(res, req)
				return
			}
			var feedData bytes.Buffer
			if err := service.DirectFetch(req.Context(), httpClient, sourceURL, &feedData); err != nil {
				slogctx.FromCtx(req.Context()).Warn("Linter failed to parse feed.",
					slog.String("url", rawURL),
					slog.Any("error", err),
				)
				m.NewPartial(templates.LinterError(fetchErr)).Handle().ServeHTTP(res, req)
				return
			}

			results, err := linter.Lint(&feedData)
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Linter failed.",
					slog.Any("error", err),
				)
				m.NewPartial(templates.LinterError(fetchErr)).Handle().ServeHTTP(res, req)
				return
			}
			m.NewPartial(templates.LinterResults(results)).Handle().ServeHTTP(res, req)
		}
	}
}
