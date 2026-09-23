// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"log/slog"
	"net/http"
	"path/filepath"

	"github.com/a-h/templ"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/pkg/htmx"
	"github.com/immanent-tech/go-base/pkg/markdownx"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/element"
)

type Help struct {
	template templ.Component
	metadata pageMetadata
}

// FullResponse renders a full page (headers, footers and list of subscriptions).
func (p *Help) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(p.template,
			templates.WithPageTitle(p.metadata.Title),
			templates.WithPageDescription(p.metadata.Description),
			templates.WithCanonicalLink(p.metadata.CanonicalLink()),
			templates.WithOpenGraphMetadata(p.metadata.OpengraphData()),
			templates.WithJSONLDSchema(
				generateSiteJSONLD(p.metadata.baseURL),
				p.metadata.JSONLD(),
			),
		)).ServeHTTP(res, req)
}

// PartialResponse will either render the list of subscriptions, the controls and update the title/dock/sidebar or, when
// paginating, just the list of subscriptions.
func (p *Help) PartialResponse(res http.ResponseWriter, req *http.Request) {
	res.Header().Set(htmx.HeaderPushURL, req.URL.String())
	templ.Handler(p.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle(p.metadata.Title)).ServeHTTP(res, req)
	templ.Handler(templates.SideBar(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
	templ.Handler(templates.Dock(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
}

// DocumentationHandler handles serving Markdown documents for help/documentation from directory in the embedded fs.
func (m *Manager) DocumentationHandler(path string) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Check, if the requested file is existing.
		contents, err := web.DocsFS.ReadFile(filepath.Join("assets", "docs", "help", "index.md"))
		if err != nil {
			// If file is not found, return HTTP 404 error.
			slogctx.FromCtx(req.Context()).Error("Could not read document.",
				slog.Any("error", err),
			)
			http.NotFound(res, req)
		}
		res.Header().Set("Cache-Control", "public, max-age=604800, s-maxage=43200")

		// Render help documentation.
		mdHTML, err := markdownx.ToHTML(contents)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Could not convert docs markdownx.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusInternalServerError)
			return
		}

		metadata := pageMetadata{
			Title: templates.PageTitle{
				Summary:     "Help & Documentation",
				Description: "RSS Reader Guides and Usage",
			},
			Description: "Get help and review documentation for using Foragd.",
			Path:        path,
			ImagePath:   "/content/logo-vertical-light.webp",
			baseURL:     m.AppConfig.GetBaseURL(),
		}

		if user := models.UserFromCtx(req.Context()); user != nil {
			RenderInternalPage(&Help{
				metadata: metadata,
				template: templates.LayoutInternal(
					&templates.InternalLayoutProps{User: user},
					templates.Document(mdHTML),
				),
			}).ServeHTTP(res, req.WithContext(req.Context()))
		} else {
			RenderExternalPage(&Help{
				metadata: metadata,
				template: templates.LayoutExternal(
					templates.Document(mdHTML),
				),
			}).ServeHTTP(res, req.WithContext(req.Context()))
		}
	}
}
