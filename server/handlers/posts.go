// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	slogctx "github.com/veqryn/slog-context"
	"github.com/yuin/goldmark/parser"
	"go.abhg.dev/goldmark/frontmatter"

	"github.com/immanent-tech/go-syndication/opengraph"
	"github.com/immanent-tech/go-syndication/rss"

	"github.com/immanent-tech/go-syndication/types"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web"
	"github.com/immanent-tech/foragd/web/templates"
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
		templates.WithOpenGraphMetadata(opengraph.New(
			title,
			"website",
			os.Getenv("FORAGD_BASEURL")+"/posts",
			os.Getenv("FORAGD_BASEURL")+"/content/logo-color.webp",
			opengraph.WithDescription(description),
			opengraph.WithSiteName(config.AppName),
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
		templates.WithOpenGraphMetadata(opengraph.New(
			p.Frontmatter.Title,
			"article",
			os.Getenv("FORAGD_BASEURL")+"/"+p.Details.Path,
			os.Getenv("FORAGD_BASEURL")+*p.Frontmatter.Image,
			opengraph.WithDescription(p.Frontmatter.Description),
			opengraph.WithSiteName(config.AppName),
			opengraph.WithAdditionalProperty("article:published_time", p.Frontmatter.CreatedAt),
			opengraph.WithAdditionalProperty("article:modified_time", *p.Frontmatter.UpdatedAt),
		)),
	)).ServeHTTP(res, req)
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

			res.Header().
				Set("Cache-Control", "public, max-age=604800, stale-while-revalidate=604800, stale-if-error=604800")

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
	buf, ok := bufPool.Get().(*bytes.Buffer)
	if !ok {
		return nil, fmt.Errorf("allocate buffer: %w", err)
	}
	defer func() {
		buf.Reset()
		bufPool.Put(buf)
	}()

	parserCtx := parser.NewContext()
	if err := mdw.Convert(contents, buf, parser.WithContext(parserCtx)); err != nil {
		return nil, fmt.Errorf("convert markdown: %w", err)
	}

	d := frontmatter.Get(parserCtx)
	var frontmatter models.MarkdownFrontMatter
	if err := d.Decode(&frontmatter); err != nil {
		return nil, fmt.Errorf("decode frontmatter: %w", err)
	}

	return &models.MarkdownFile{
		Frontmatter: frontmatter,
		Details:     details,
		Content:     buf.Bytes(),
	}, nil
}

// HandlePostsFeed handles showing an RSS file for posts.
func HandlePostsFeed() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Check, if the requested file is existing.
		posts, err := getPosts()
		if err != nil {
			// If file is not found, return HTTP 404 error.
			slogctx.FromCtx(req.Context()).Error("Could not read post document.",
				slog.Any("error", err),
			)
			http.NotFound(res, req)
			return
		}

		baseURL := os.Getenv("FORAGD_BASEURL")
		if baseURL == "" {
			// If file is not found, return HTTP 404 error.
			slogctx.FromCtx(req.Context()).Error("Could not read base URL from env.",
				slog.Any("error", err),
			)
			http.NotFound(res, req)
			return
		}

		// Generate RSS file.
		rssFile := rss.NewRSS(
			"Posts from the Foragd Team",
			"Comparisons, opinions and other content from the Foragd team",
			baseURL,
			rss.WithCopyright("Copyright 2026 Joshua Rich <joshua.rich@gmail.com>"),
			rss.WithManagingEditor("hello@immanent.tech (Immanent Tech)"),
			rss.WithWebmaster("hello@immanent.tech (Immanent Tech)"),
			rss.WithChannelLanguage("en-us"),
			rss.WithChannelImage(&rss.Image{
				Link:  baseURL,
				URL:   baseURL + "/content/logo-color.webp",
				Title: "Foragd Logo",
			}),
			rss.WithUpdatePeriod("monthly"),
		)
		for post := range slices.Values(posts.Files) {
			data, err := readPost(post)
			if err != nil {
				continue
			}
			var published time.Time
			if published, err = time.Parse(time.DateOnly, data.Frontmatter.CreatedAt); err != nil {
				slogctx.FromCtx(req.Context()).Warn("Unable to parse published date of post.",
					slog.String("file", post.File),
					slog.Any("error", err),
				)
			}

			// Generate item for post.
			item := rss.NewItem(
				rss.WithItemTitle(data.Frontmatter.Title),
				rss.WithItemDescription(data.Frontmatter.Description),
				rss.WithItemLink(baseURL+"/posts/"+data.Details.Path),
				rss.WithItemGUID(rss.GenerateGUID(baseURL+"/posts/"+data.Details.Path, true)),
				rss.WithItemImage(&types.ImageInfo{
					Title: data.Frontmatter.Title,
					URL:   baseURL + *data.Frontmatter.Image,
				}),
				rss.WithItemContent(data.Content),
				rss.WithItemPublishedDate(published),
			)
			rssFile.Channel.Items = append(rssFile.Channel.Items, *item)
		}

		slices.SortFunc(rssFile.Channel.Items, func(a rss.Item, b rss.Item) int {
			return a.GetUpdatedDate().Compare(b.GetUpdatedDate())
		})
		slices.Reverse(rssFile.Channel.Items)

		// Write RSS file in response.
		res.Header().Set("Cache-Control", "public, max-age=3600, stale-while-revalidate=3600, stale-if-error=86400")
		res.Header().Set("Content-Type", types.MimeTypesRSS[0])
		if _, err := res.Write([]byte(xml.Header)); err != nil {
			slogctx.FromCtx(req.Context()).Error("Could not write xml header to response.",
				slog.Any("error", err),
			)
		}
		enc := xml.NewEncoder(res)
		if err := enc.Encode(rssFile); err != nil {
			slogctx.FromCtx(req.Context()).Warn("Could write RSS content to response.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}
