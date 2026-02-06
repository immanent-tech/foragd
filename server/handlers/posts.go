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
	"github.com/yuin/goldmark/parser"
	"go.abhg.dev/goldmark/frontmatter"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/helpers/opengraph"
)

var postsPath = "assets/docs/posts"

var getPosts = sync.OnceValues(func() (*models.FileIndex, error) {
	var posts models.FileIndex
	if _, err := toml.DecodeFS(web.DocsFS, filepath.Join(postsPath, "directory.toml"), &posts); err != nil {
		return nil, fmt.Errorf("read posts docs directory.toml: %w", err)
	}
	return &posts, nil
})

// PostsIndex is the index of all posts.
type PostsIndex struct {
	posts []*models.MarkdownFile
}

// FullResponse renders the posts index.
func (p *PostsIndex) FullResponse(res http.ResponseWriter, req *http.Request) {
	title := "Posts from the Foragd Team"
	description := "Comparisons, opinions and other content from the Foragd team"
	templ.Handler(templates.CreatePage(
		templates.PostsIndex(p.posts),
		templates.WithPageTitle(title),
		templates.WithPageDescription(description),
		templates.WithOGMetadata(opengraph.NewMetadata(
			opengraph.WithTitle(title),
			opengraph.WithDescription(description),
		)),
	),
	).ServeHTTP(res, req)
}

// Post is an individual post.
type Post struct {
	*models.MarkdownFile
}

// FullResponse renders an individual post.
func (p *Post) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(templates.CreatePage(
		templates.Post(p.MarkdownFile),
		templates.WithPageTitle(p.Frontmatter.Title),
		templates.WithPageDescription(p.Frontmatter.Description),
		templates.WithOGMetadata(opengraph.NewMetadata(
			opengraph.WithTitle(p.Frontmatter.Title),
			opengraph.WithDescription(p.Frontmatter.Description),
			opengraph.WithType("article", map[string]string{
				"article:published_time": p.Frontmatter.CreatedAt,
				"article:modified_time":  p.Frontmatter.UpdatedAt,
			}),
		)),
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
			return
		}

		// Show index when no specific post has been requested.
		switch doc {
		case "":
			// Posts index.
			index := &PostsIndex{posts: make([]*models.MarkdownFile, 0, len(posts.Files))}
			for file := range slices.Values(posts.Files) {
				post, err := readPost(file)
				if err != nil {
					slogctx.FromCtx(req.Context()).Warn("Could not read post details.",
						slog.String("file", file.File),
						slog.Any("error", err),
					)
					continue
				}
				index.posts = append(index.posts, post)
			}
			RenderExternalPage(index).ServeHTTP(res, req)
		default:
			// Individual post.

			idx := slices.IndexFunc(posts.Files, func(e models.FileDetails) bool {
				return strings.HasPrefix(e.File, doc)
			})
			if idx == -1 {
				res.WriteHeader(http.StatusNotFound)
				return
			}

			res.Header().Set("Cache-Control", "public, max-age=604800, s-maxage=43200")

			post, err := readPost(posts.Files[idx])
			if err != nil {
				// If file is not found, return HTTP 404 error.
				slogctx.FromCtx(req.Context()).Error("Could not read post document.",
					slog.String("doc", doc),
					slog.Any("error", err),
				)
				res.WriteHeader(http.StatusInternalServerError)
				return
			}
			RenderExternalPage(&Post{MarkdownFile: post}).ServeHTTP(res, req)
		}
	}
}

func readPost(details models.FileDetails) (*models.MarkdownFile, error) {
	contents, err := web.DocsFS.ReadFile(filepath.Join(postsPath, details.File))
	if err != nil {
		return nil, fmt.Errorf("read file contents: %w", err)
	}

	mdw := loadMarkdownWriter()
	postBuf, ok := bufPool.Get().(*bytes.Buffer)
	if !ok {
		return nil, fmt.Errorf("allocate buffer: %w", err)
	}
	postBuf.Reset()
	defer bufPool.Put(postBuf)

	parserCtx := parser.NewContext()
	if err := mdw.Convert(contents, postBuf, parser.WithContext(parserCtx)); err != nil {
		return nil, fmt.Errorf("convert markdown: %w", err)
	}

	d := frontmatter.Get(parserCtx)
	var fm models.MarkdownFrontMatter
	if err := d.Decode(&fm); err != nil {
		return nil, fmt.Errorf("decode frontmatter: %w", err)
	}

	return &models.MarkdownFile{
		Frontmatter: fm,
		Details:     details,
		Content:     postBuf.Bytes(),
	}, nil
}
