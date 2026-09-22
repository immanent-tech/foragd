// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"log/slog"
	"net/http"
	"slices"
	"sync"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/immanent-tech/go-base/pkg/markdownx"

	"github.com/immanent-tech/foragd/web"
	"github.com/immanent-tech/foragd/web/templates"
)

var getComparisons = sync.OnceValues(func() ([]*markdownx.File, error) {
	var postsPath = "assets/docs/comparisons"
	return markdownx.ReadDir(web.DocsFS, postsPath)
})

type ComparisonPage struct {
	text     *markdownx.File
	metadata pageMetadata
}

func (p *ComparisonPage) FullResponse(res http.ResponseWriter, req *http.Request) {
	// Render appropriate content.
	templ.Handler(
		templates.CreatePage(templates.Comparison(p.text),
			templates.WithPageTitle(p.metadata.Title),
			templates.WithPageDescription(p.metadata.Description),
			templates.WithCanonicalLink(p.metadata.CanonicalLink()),
			templates.WithOpenGraphMetadata(p.metadata.OpengraphData()),
			templates.WithJSONLDSchema(
				generateSiteJSONLD(p.metadata.baseURL),
				p.metadata.JSONLD(),
			),
		),
	).ServeHTTP(res, req)
}

func HandleComparison(appCfg AppConfig) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Check, if the requested file is existing.
		comparisons, err := getComparisons()
		if err != nil {
			// If file is not found, return HTTP 404 error.
			slogctx.FromCtx(req.Context()).Error("Could not read comparisons.",
				slog.Any("error", err),
			)
			http.NotFound(res, req)
			return
		}

		// Get the comparison document.
		service := chi.RouteContext(req.Context()).URLParam("service")
		idx := slices.IndexFunc(comparisons, func(p *markdownx.File) bool {
			return p.Frontmatter.Slug == service
		})
		if idx == -1 {
			res.WriteHeader(http.StatusNotFound)
			return
		}

		res.Header().
			Set("Cache-Control", "public, max-age=604800, stale-while-revalidate=604800, stale-if-error=604800")

		caser := cases.Title(language.English)
		text := comparisons[idx]

		RenderExternalPage(&ComparisonPage{
			text: text,
			// Generate a page title and description.
			metadata: pageMetadata{
				Title: templates.PageTitle{
					Summary:     text.Frontmatter.Description,
					Description: text.Frontmatter.Description,
					Date:        text.Frontmatter.CreatedAt,
				},
				Description: "A detailed comparison of Foragd and " + caser.String(
					text.Frontmatter.Slug,
				) + " covering pricing, features, and which is best for different use cases.",
				Path:      text.Frontmatter.Slug,
				ImagePath: "/content/logo-vertical-light.webp",
				baseURL:   appCfg.GetBaseURL(),
			},
		}).ServeHTTP(res, req)
	}
}
