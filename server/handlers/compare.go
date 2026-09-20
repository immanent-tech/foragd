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
	text *markdownx.File
}

func (p *ComparisonPage) FullResponse(res http.ResponseWriter, req *http.Request) {
	caser := cases.Title(language.English)

	// Generate a page title and description.
	metadata := &pageMetadata{
		Title: templates.PageTitle{
			Summary:     p.text.Frontmatter.Description,
			Description: p.text.Frontmatter.Description,
			Date:        p.text.Frontmatter.CreatedAt,
		},
		Description: "A detailed comparison of Foragd and " + caser.String(
			p.text.Frontmatter.Slug,
		) + " covering pricing, features, and which is best for different use cases.",
		Path:      req.URL.Path,
		ImagePath: "/content/logo-vertical-light.webp",
	}

	// Render appropriate content.
	templ.Handler(
		templates.CreatePage(templates.Comparison(p.text),
			templates.WithPageTitle(metadata.Title),
			templates.WithPageDescription(metadata.Description),
			templates.WithCanonicalLink(metadata.CanonicalLink(req)),
			templates.WithOpenGraphMetadata(metadata.OpengraphData(req)),
			templates.WithJSONLDSchema(
				generateSiteJSONLD(req),
				metadata.JSONLD(req),
			),
		),
	).ServeHTTP(res, req)
}

func HandleComparison() http.HandlerFunc {
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

		RenderExternalPage(&ComparisonPage{
			text: comparisons[idx],
		}).ServeHTTP(res, req)
	}
}
