// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"bytes"
	"log/slog"
	"net/http"
	"path/filepath"

	"github.com/a-h/templ"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/web"
	"github.com/immanent-tech/foragd/web/templates"
)

// DocumentationHandler handles serving Markdown documents for help/documentation from directory in the embedded fs.
func DocumentationHandler() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// doc := chi.URLParam(req, "*")
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

		mdw := loadMarkdownWriter()

		docsBuf, ok := bufPool.Get().(*bytes.Buffer)
		if !ok {
			res.WriteHeader(http.StatusInternalServerError)
			slogctx.FromCtx(req.Context()).Error("Could not write docs.")
			return
		}
		docsBuf.Reset()
		defer bufPool.Put(docsBuf)

		if err := mdw.Convert(contents, docsBuf); err != nil {
			slogctx.FromCtx(req.Context()).Error("Could not convert docs markdown.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusInternalServerError)
			return
		}

		template := templates.CreatePage(
			templates.Document(docsBuf.Bytes()),
			templates.WithPageTitle("Documentation"),
		)
		templ.Handler(template).ServeHTTP(res, req)
	}
}
