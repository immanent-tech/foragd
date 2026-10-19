// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"log/slog"
	"net/http"
	"path/filepath"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/pkg/markdownx"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web"
	"github.com/immanent-tech/foragd/web/templates"
)

type Help struct {
	*Page
}

// DocumentationHandler handles serving Markdown documents for help/documentation from directory in the embedded fs.
func (m *Manager) DocumentationHandler(path string) http.HandlerFunc {
	const docPrefix = "files/docs/help"
	page := &Help{
		Page: m.NewPage(path,
			WithSummary("Help & Documentation"),
			WithDescription("Foragd RSS reader help guides and usage"),
			WithMetadataDescription("Get help and read documentation for using Foragd."),
		),
	}
	return func(res http.ResponseWriter, req *http.Request) {
		// Check, if the requested file is existing.
		contents, err := web.Files.ReadFile(filepath.Join(docPrefix, "index.md"))
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

		if user := models.UserFromCtx(req.Context()); user != nil {
			page.template = templates.LayoutInternal(
				"",
				&templates.InternalLayoutProps{User: user},
				templates.Document(mdHTML),
			)
		} else {
			page.template = templates.LayoutExternal(
				templates.Document(mdHTML),
			)
		}
		page.Render().ServeHTTP(res, req.WithContext(req.Context()))
	}
}
