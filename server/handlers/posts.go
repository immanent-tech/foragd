// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/http"
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
	"github.com/immanent-tech/foragd/pkg/formats/markdown"
	"github.com/immanent-tech/foragd/web"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/partials"
	"github.com/immanent-tech/foragd/web/templates/slots"
)

var postsPath = "assets/docs/blog"

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
	title := "RSS Feed Reader Blog — Tips, Guides and Comparisons"
	description := "Guides, comparisons and tips on RSS feed readers, finding feeds, managing information overload, and taking back control of your reading from social media algorithms."
	templ.Handler(templates.CreatePage(
		templates.PostsIndex(p.posts),
		templates.WithPageTitle(title),
		templates.WithPageDescription(description),
		templates.WithCanonicalLink(config.GetBaseURL()+"/blog"),
		templates.WithOpenGraphMetadata(opengraph.New(
			title,
			"website",
			config.GetBaseURL()+"/blog",
			config.GetBaseURL()+"/content/logo-vertical-light.webp",
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
	ctx := slots.WithSlot(
		req.Context(),
		slots.Header,
		partials.RenderJSONLD(strings.ToLower(strings.ReplaceAll(p.Frontmatter.Title, " ", "")), *p.JsonLD),
	)
	templ.Handler(templates.CreatePage(
		templates.Post(p.MarkdownFile),
		templates.WithPageTitle(p.Frontmatter.PageTitle),
		templates.WithPageDescription(p.Frontmatter.Description),
		templates.WithCanonicalLink(config.GetBaseURL()+"/blog/"+p.Details.Path),
		templates.WithOpenGraphMetadata(opengraph.New(
			p.Frontmatter.PageTitle,
			"article",
			config.GetBaseURL()+"/blog/"+p.Details.Path,
			config.GetBaseURL()+*p.Frontmatter.Image,
			opengraph.WithDescription(p.Frontmatter.Description),
			opengraph.WithSiteName(config.AppName),
			opengraph.WithAdditionalProperty("article:published_time", p.Frontmatter.GetCreatedDate()),
			opengraph.WithAdditionalProperty("article:modified_time", p.Frontmatter.GetUpdatedDate()),
		)),
	)).ServeHTTP(res, req.WithContext(ctx))
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

	mdw := markdown.LoadMarkdownWriter()
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

	jsonld, err := generateJSONLD(&frontmatter, &details)
	if err != nil {
		return nil, fmt.Errorf("generate json-ld data: %w", err)
	}

	return &models.MarkdownFile{
		Frontmatter: frontmatter,
		Details:     details,
		JsonLD:      &jsonld,
		Content:     buf.Bytes(),
	}, nil
}

func generateJSONLD(fm *models.MarkdownFrontMatter, details *models.FileDetails) (json.RawMessage, error) {
	data := map[string]any{
		"@context":      "https://schema.org",
		"@type":         "Article",
		"headline":      fm.Title,
		"description":   fm.Description,
		"image":         config.GetBaseURL() + *fm.Image,
		"datePublished": fm.GetCreatedDate(),
		"dateModified":  fm.GetUpdatedDate(),
		"author": map[string]any{
			"@type": "Person",
			"name":  fm.Author,
		},
		"publisher": map[string]any{
			"@type": "Organization",
			"name":  "Foragd",
			"url":   config.GetBaseURL(),
		},
		"mainEntityOfPage": map[string]any{
			"@type": "WebPage",
			"@id":   config.GetBaseURL() + "/blog/" + details.Path,
		},
	}
	return json.Marshal(data)
}

// HandlePostsFeed handles showing an RSS file for posts.
func HandlePostsFeed() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Reject requests with any query parameters set.
		if len(req.URL.Query()) > 0 {
			res.WriteHeader(http.StatusBadRequest)
			return
		}
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

		// Generate RSS file.
		rssFile := rss.NewRSS(
			"Posts from the Foragd Team",
			"Comparisons, opinions and other content from the Foragd team",
			config.GetBaseURL(),
			rss.WithCopyright("Copyright 2026 Joshua Rich <joshua.rich@gmail.com>"),
			rss.WithManagingEditor("hello@immanent.tech (Immanent Tech)"),
			rss.WithWebmaster("hello@immanent.tech (Immanent Tech)"),
			rss.WithChannelLanguage("en-us"),
			rss.WithChannelImage(&rss.Image{
				Link:  config.GetBaseURL(),
				URL:   config.GetBaseURL() + "/content/logo-vertical-light.webp",
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
			if published, err = time.Parse(time.DateOnly, data.Frontmatter.GetCreatedDate()); err != nil {
				slogctx.FromCtx(req.Context()).Warn("Unable to parse published date of post.",
					slog.String("file", post.File),
					slog.Any("error", err),
				)
			}

			// Generate item for post.
			item := rss.NewItem(
				rss.WithItemTitle(data.Frontmatter.Title),
				rss.WithItemDescription(data.Frontmatter.Description),
				rss.WithItemLink(config.GetBaseURL()+"/blog/"+data.Details.Path),
				rss.WithItemGUID(rss.GenerateGUID(config.GetBaseURL()+"/blog/"+data.Details.Path, true)),
				rss.WithItemImage(&types.ImageInfo{
					Title: data.Frontmatter.Title,
					URL:   config.GetBaseURL() + *data.Frontmatter.Image,
				}),
				rss.WithItemContent(data.Content),
				rss.WithItemPublishedDate(published),
			)
			rssFile.Channel.Items = append(rssFile.Channel.Items, *item)
		}

		slices.SortFunc(rssFile.Channel.Items, func(a rss.Item, b rss.Item) int {
			return a.GetPublishedDate().Compare(*b.GetPublishedDate())
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
