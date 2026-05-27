// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/immanent-tech/go-syndication/opengraph"
	slogctx "github.com/veqryn/slog-context"
	"github.com/yuin/goldmark/parser"
	fm "go.abhg.dev/goldmark/frontmatter"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/pkg/formats/markdown"
	"github.com/immanent-tech/foragd/web"
	"github.com/immanent-tech/foragd/web/templates"
)

var getPolicyDocs = sync.OnceValues(func() (*models.FileIndex, error) {
	var policies models.FileIndex
	if _, err := toml.DecodeFS(web.DocsFS, "assets/docs/policies/directory.toml", &policies); err != nil {
		return nil, fmt.Errorf("read policy docs directory.toml: %w", err)
	}
	return &policies, nil
})

// PolicyDocsHandler handles serving policy Markdown documents from directory in the embedded fs.
func PolicyDocsHandler() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		doc := chi.URLParam(req, "*")
		// Check, if the requested file is existing.
		polices, err := getPolicyDocs()
		if err != nil {
			// If file is not found, return HTTP 404 error.
			slogctx.FromCtx(req.Context()).Error("Could not read policy document.",
				slog.String("doc", doc),
				slog.Any("error", err),
			)
			http.NotFound(res, req)
		}

		idx := slices.IndexFunc(polices.Files, func(e models.FileDetails) bool {
			return strings.HasPrefix(e.File, doc)
		})
		if idx == -1 {
			res.WriteHeader(http.StatusNotFound)
			return
		}

		metadata := polices.Files[idx]
		contents, err := web.DocsFS.ReadFile(filepath.Join("assets/docs/policies", metadata.File))
		if err != nil {
			// If file is not found, return HTTP 404 error.
			slogctx.FromCtx(req.Context()).Error("Could not read policy document.",
				slog.String("doc", doc),
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusInternalServerError)
			return
		}

		mdw := markdown.LoadMarkdownWriter()

		policyBuf, ok := bufPool.Get().(*bytes.Buffer)
		if !ok {
			res.WriteHeader(http.StatusInternalServerError)
			slogctx.FromCtx(req.Context()).Error("Could not write policy.")
			return
		}
		policyBuf.Reset()
		defer bufPool.Put(policyBuf)

		parserCtx := parser.NewContext()
		if err := mdw.Convert(contents, policyBuf, parser.WithContext(parserCtx)); err != nil {
			slogctx.FromCtx(req.Context()).Error("Could not convert policy markdown.",
				slog.String("doc", doc),
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusInternalServerError)
			return
		}

		d := fm.Get(parserCtx)
		var frontmatter models.MarkdownFrontMatter
		if err := d.Decode(&frontmatter); err != nil {
			slogctx.FromCtx(req.Context()).Error("Could not convert policy markdown.",
				slog.String("doc", doc),
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusInternalServerError)
			return
		}

		res.Header().Set("Cache-Control", "public, max-age=604800, s-maxage=43200")
		template := templates.CreatePage(
			templates.LayoutExternal(
				templates.Document(policyBuf.Bytes()),
			),
			templates.WithPageTitle(frontmatter.Title),
			templates.WithPageDescription(frontmatter.Description),
			templates.WithOpenGraphMetadata(opengraph.New(
				frontmatter.Title,
				"website",
				config.GetBaseURL()+"/"+metadata.Path,
				config.GetBaseURL()+"/content/logo-vertical-light.webp",
				opengraph.WithDescription(frontmatter.Description),
				opengraph.WithSiteName(config.AppName),
			)),
		)
		templ.Handler(template).ServeHTTP(res, req)
	}
}
