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

// PostsIndex is the index of all posts.
type PostsIndex struct {
	posts *models.DocsDirectory
}

// FullResponse renders the posts index.
func (p *PostsIndex) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(templates.CreatePage(
		templates.PostsIndex(p.posts),
		templates.WithPageTitle("Posts from the Foragd Team"),
		templates.WithPageDescription("Comparisons, opinions and other content from the Foragd team"),
	),
	).ServeHTTP(res, req)
}

// Post is an individual post.
type Post struct {
	content  []byte
	metadata *models.DocMetadata
}

// FullResponse renders an individual post.
func (p *Post) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(templates.CreatePage(
		templates.Document(p.content),
		templates.WithPageTitle(p.metadata.Title),
		templates.WithPageDescription(p.metadata.Description),
	),
	).ServeHTTP(res, req)
}

// HandlePosts handles showing the posts index or individual posts.
func HandlePosts() http.HandlerFunc {
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
			RenderExternalPage(&PostsIndex{
				posts: posts,
			}).ServeHTTP(res, req)
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

		mdw := loadMarkdownWriter()

		postBuf, ok := bufPool.Get().(*bytes.Buffer)
		if !ok {
			res.WriteHeader(http.StatusInternalServerError)
			slogctx.FromCtx(req.Context()).Error("Could not write post.")
			return
		}
		postBuf.Reset()
		defer bufPool.Put(postBuf)

		if err := mdw.Convert(contents, postBuf); err != nil {
			slogctx.FromCtx(req.Context()).Error("Could not convert post markdown.",
				slog.String("doc", doc),
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusInternalServerError)
			return
		}

		RenderExternalPage(&Post{
			content:  postBuf.Bytes(),
			metadata: &metadata,
		}).ServeHTTP(res, req)
	}
}
