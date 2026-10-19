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

	"github.com/immanent-tech/go-base/pkg/markdownx"

	"github.com/immanent-tech/foragd/web"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/components"
)

var getPolicyDocs = sync.OnceValues(func() ([]*markdownx.File, error) {
	var policiesPath = "files/docs/policies"
	return markdownx.ReadDir(web.Files, policiesPath)
})

type Policy struct {
	*Page

	policyOG     *opengraph.Article
	policyJsonLd *schemaorg.Article
}

func (p Policy) ToMetaTags() templ.Component {
	return p.policyOG.ToMetaTags()
}

// PolicyDocsHandler handles serving policy Markdown documents from directory in the embedded fs.
func (m *Manager) PolicyDocsHandler() http.HandlerFunc {
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

		idx := slices.IndexFunc(polices, func(p *markdownx.File) bool {
			return chi.URLParam(req, "*") == p.Frontmatter.Slug
		})
		if idx == -1 {
			res.WriteHeader(http.StatusNotFound)
			return
		}

		policyFile := polices[idx]
		path := m.AppConfig.GetBaseURL().JoinPath(policyFile.Frontmatter.Slug).String()
		page := &Policy{
			Page: m.NewPage(path,
				WithSummary(policyFile.Frontmatter.Title),
				WithDescription("Service Policy"),
				WithTemplate(templates.LayoutExternal(
					templates.Document(policyFile.Content),
				)),
			),
			policyOG: opengraph.NewArticle(
				policyFile.Frontmatter.Title+" Service Policy",
				path,
				policyFile.Frontmatter.Description,
				m.AppConfig.GetBaseURL().JoinPath("/images/logo-vertical-light.webp").String(),
				policyFile.Frontmatter.GetCreatedDate().String(),
				policyFile.Frontmatter.GetUpdatedDate().String(),
				"",
				[]string{"Immanent Tech <hello@immanent.tech>"},
				"Policies",
				nil,
			),
			policyJsonLd: schemaorg.NewArticle(
				policyFile.Frontmatter.Title+" Service Policy",
				nil,
				nil,
				components.OrgJSONLD,
				policyFile.Frontmatter.GetCreatedDate().String(),
				policyFile.Frontmatter.GetUpdatedDate().String(),
				policyFile.Frontmatter.Description,
			),
		}
		template := templ.Join(page.policyJsonLd.ToJsonLd(), templates.CreatePage(page.appCfg, page.sessionMgr, page))

		res.Header().Set("Cache-Control", "public, max-age=604800, s-maxage=43200")
		templ.Handler(template).ServeHTTP(res, req)
	}
}
