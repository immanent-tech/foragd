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
	"github.com/indaco/teseo/opengraph"
	"github.com/indaco/teseo/schemaorg"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/pkg/formats/markdown"
	"github.com/immanent-tech/foragd/web"
	"github.com/immanent-tech/foragd/web/templates"
)

var getPolicyDocs = sync.OnceValues(func() ([]*markdown.File, error) {
	var policiesPath = "assets/docs/policies"
	return markdown.ReadDir(web.DocsFS, policiesPath)
})

// PolicyDocsHandler handles serving policy Markdown documents from directory in the embedded fs.
func PolicyDocsHandler() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Check, if the requested file is existing.
		polices, err := getPolicyDocs()
		if err != nil {
			// If file is not found, return HTTP 404 error.
			slogctx.FromCtx(req.Context()).Error("Could not read policy documents.",
				slog.Any("error", err),
			)
			http.NotFound(res, req)
		}

		idx := slices.IndexFunc(polices, func(p *markdown.File) bool {
			return chi.URLParam(req, "*") == p.Frontmatter.Slug
		})
		if idx == -1 {
			res.WriteHeader(http.StatusNotFound)
			return
		}

		policyFile := polices[idx]

		res.Header().Set("Cache-Control", "public, max-age=604800, s-maxage=43200")
		title := templates.PageTitle{
			Summary:     policyFile.Frontmatter.Title,
			Description: "Service Policy",
		}
		policyOG := opengraph.NewArticle(
			title.String(),
			config.GetBaseURL()+"/"+policyFile.Frontmatter.Slug,
			policyFile.Frontmatter.Description,
			config.GetBaseURL()+"/content/logo-vertical-light.webp",
			policyFile.Frontmatter.GetCreatedDate().String(),
			policyFile.Frontmatter.GetUpdatedDate().String(),
			"",
			[]string{"Immanent Tech <hello@immanent.tech>"},
			"Policies",
			nil,
		)
		policyJsonLd := schemaorg.NewArticle(
			title.String(),
			nil,
			nil,
			orgJsonLd,
			policyFile.Frontmatter.GetCreatedDate().String(),
			policyFile.Frontmatter.GetUpdatedDate().String(),
			policyFile.Frontmatter.Description,
		)
		template := templates.CreatePage(
			templates.LayoutExternal(
				templates.Document(policyFile.Content),
			),
			templates.WithPageTitle(title),
			templates.WithPageDescription(policyFile.Frontmatter.Description),
			templates.WithOpenGraphMetadata(policyOG),
			templates.WithJSONLDSchema(
				websiteJsonLd,
				policyJsonLd,
			),
		)
		templ.Handler(template).ServeHTTP(res, req)
	}
}
