// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/a-h/templ"
	"github.com/go-resty/resty/v2"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/validation"
	"github.com/immanent-tech/go-syndication/linter"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/service"
	"github.com/immanent-tech/foragd/web/templates"
)

type Linter struct {
	pageMetadata

	svc pageServices
}

func (p *Linter) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(
			p.svc.appCfg,
			p.svc.sessionMgr,
			templates.Linter(),
			templates.WithPageTitle(p.Title),
			templates.WithPageDescription(p.Description),
			templates.WithCanonicalLink(p.CanonicalLink()),
			templates.WithOpenGraphMetadata(p.OpengraphData()),
			templates.WithJSONLDSchema(
				generateSiteJSONLD(p.baseURL),
				p.JSONLD(),
			),
		),
	).ServeHTTP(res, req)
}

type LinterResponse struct {
	Results map[string][]linter.Result
}

func (p *LinterResponse) PartialResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(templates.LinterResults(p.Results)).ServeHTTP(res, req)
}

type LinterError struct {
	msg *models.UserMessage
}

func (p *LinterError) PartialResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(templates.LinterError(p.msg)).ServeHTTP(res, req)
}

func (m *Manager) HandleLinter(httpClient *resty.Client) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		switch fetchErr := models.NewErrorMessage(
			"Unable to find feed at provided URL",
			"No feed details could be fetched from the given URL. This could be a temporary error.",
		); req.Method {
		case http.MethodGet:
			RenderExternalPage(&Linter{
				Title: templates.PageTitle{
					Summary:     "Free RSS Feed Linter",
					Description: "Lint Any Website's Feed to check for validation errors and recommendations",
				},
				Description: "Foragd's free feed RSS linter instantly shows whether a site's feed passes validation and contains recommended values to ensure maximum compatibility",
				Path:        "/linter",
				ImagePath:   "/images/logo-vertical-light.webp",
				baseURL:     m.AppConfig.GetBaseURL(),
				svc:         m.NewPageServices(),
			}).ServeHTTP(res, req)
		case http.MethodPost:
			rawURL, err := validation.SanitizeURLString(req.FormValue("url"))
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Linter failed to parse feed.",
					slog.String("url", rawURL),
					slog.Any("error", err),
				)
				RenderPartial(&LinterError{
					msg: fetchErr,
				}).ServeHTTP(res, req)
				return
			}
			sourceURL, err := url.Parse(rawURL)
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Linter failed to parse feed.",
					slog.String("url", rawURL),
					slog.Any("error", err),
				)
				RenderPartial(&LinterError{
					msg: fetchErr,
				}).ServeHTTP(res, req)
				return
			}
			var feedData bytes.Buffer
			if err := service.DirectFetch(req.Context(), httpClient, sourceURL, &feedData); err != nil {
				slogctx.FromCtx(req.Context()).Warn("Linter failed to parse feed.",
					slog.String("url", rawURL),
					slog.Any("error", err),
				)
				RenderPartial(&LinterError{
					msg: fetchErr,
				}).ServeHTTP(res, req)
				return
			}

			results, err := linter.Lint(&feedData)
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Linter failed.",
					slog.Any("error", err),
				)
				RenderPartial(&LinterError{
					msg: fetchErr,
				}).ServeHTTP(res, req)
				return
			}
			RenderPartial(&LinterResponse{Results: results}).ServeHTTP(res, req)
		}
	}
}
