// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/go-chi/chi/v5"
	"github.com/russross/blackfriday/v2"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web"
	"github.com/immanent-tech/foragd/web/templates"
)

var postsPath = "assets/docs/posts"

var getPosts = sync.OnceValues(func() (*models.DocsDirectory, error) {
	var posts models.DocsDirectory
	if _, err := toml.DecodeFS(web.DocsFS, filepath.Join(postsPath, "directory.toml"), &posts); err != nil {
		return nil, fmt.Errorf("read posts docs directory.toml: %w", err)
	}
	return &posts, nil
})

// PostsHandler handles showing posts.
func PostsHandler() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		doc := chi.URLParam(req, "*")
		// Check, if the requested file is existing.
		posts, err := getPosts()
		if err != nil {
			// If file is not found, return HTTP 404 error.
			slogctx.FromCtx(req.Context()).Error("Could not read post document.",
				slog.String("doc", doc),
				slog.Any("error", err),
			)
			http.NotFound(res, req)
		}

		// Show index when no specific post has been requested.
		if doc == "" {
			renderPage(templates.NewPage(templates.PostsIndex(posts))).ServeHTTP(res, req)
			return
		}

		idx := slices.IndexFunc(posts.Docs, func(e models.DocMetadata) bool {
			return strings.HasPrefix(e.File, doc)
		})

		if idx == -1 {
			res.WriteHeader(http.StatusNotFound)
			return
		}

		metadata := posts.Docs[idx]
		contents, err := web.DocsFS.ReadFile(filepath.Join(postsPath, metadata.File))
		if err != nil {
			// If file is not found, return HTTP 404 error.
			slogctx.FromCtx(req.Context()).Error("Could not read post document.",
				slog.String("doc", doc),
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusInternalServerError)
			return
		}

		res.Header().Set("Cache-Control", "public, max-age=604800, s-maxage=43200")
		output := blackfriday.Run(contents, blackfriday.WithExtensions(blackfriday.AutoHeadingIDs))
		template := templates.NewPage(
			templates.Document(output),
			templates.WithPageTitle(metadata.Title),
			templates.WithPageDescription(metadata.Description),
		).FullTemplate()
		err = template.Render(req.Context(), res)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Could not render post.",
				slog.String("doc", doc),
				slog.Any("error", err),
			)
		}
	}
}
