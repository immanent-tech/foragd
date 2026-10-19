// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"encoding/xml"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/indaco/teseo/opengraph"
	"github.com/indaco/teseo/schemaorg"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/pkg/markdownx"
	feeds "github.com/immanent-tech/go-syndication"
	"github.com/immanent-tech/go-syndication/atom"
	"github.com/immanent-tech/go-syndication/rss"

	"github.com/immanent-tech/go-syndication/types"

	"github.com/immanent-tech/foragd/web"
	"github.com/immanent-tech/foragd/web/templates"
)

var getPosts = sync.OnceValues(func() ([]*markdownx.File, error) {
	var postsPath = "files/docs/blog"
	return markdownx.ReadDir(web.Files, postsPath)
})

// PostsIndex is the index of all posts.
type PostsIndex struct {
	*Page
}

// Post is an individual post.
type Post struct {
	*Page
	*markdownx.File

	postOG     *opengraph.Article
	postJsonLd *schemaorg.Article
}

func (p Post) ToMetaTags() templ.Component {
	return p.postOG.ToMetaTags()
}

// HandlePosts handles showing the posts index or individual posts.
func (m *Manager) HandlePosts() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Check, if the requested file is existing.
		posts, err := getPosts()
		if err != nil {
			// If file is not found, return HTTP 404 error.
			slogctx.FromCtx(req.Context()).Error("Could not read posts.",
				slog.Any("error", err),
			)
			http.NotFound(res, req)
			return
		}

		// Sort by created date (most recent to least recent).
		slices.SortFunc(posts, func(a, b *markdownx.File) int {
			return a.Frontmatter.GetCreatedDate().Compare(b.Frontmatter.GetCreatedDate())
		})
		slices.Reverse(posts)

		// Show index when no specific post has been requested.
		switch slug := chi.URLParam(req, "*"); slug {
		case "":
			// Posts index.
			page := &PostsIndex{
				Page: m.NewPage(
					"/blog",
					WithSummary("Blog"),
					WithDescription("RSS Reader Tips, Guides and Comparisons"),
					WithMetadataDescription(
						"Guides, comparisons and tips on RSS feed readers, finding feeds, managing information overload, and taking back control of your reading from social media algorithms.",
					),
					WithTemplate(templates.PostsIndex(templates.PostsData{
						Files:   posts,
						BaseURL: m.AppConfig.GetBaseURL(),
					})),
				),
			}
			page.Render().ServeHTTP(res, req)
		default:
			// Individual post.
			idx := slices.IndexFunc(posts, func(p *markdownx.File) bool {
				return p.Frontmatter.Slug == slug
			})
			if idx == -1 {
				res.WriteHeader(http.StatusNotFound)
				return
			}

			file := posts[idx]
			path := m.AppConfig.GetBaseURL().JoinPath("blog", file.Frontmatter.Slug).String()
			page := &Post{
				File: file,
				Page: m.NewPage(path,
					WithSummary(file.Frontmatter.Title),
					WithDescription("Blog"),
					WithDate(file.Frontmatter.GetCreatedDate().Format(time.DateOnly)),
				),
				postOG: opengraph.NewArticle(
					file.Frontmatter.Title+" Blog",
					path,
					file.Frontmatter.Description,
					m.AppConfig.GetBaseURL().JoinPath(*file.Frontmatter.Image).String(),
					file.Frontmatter.GetCreatedDate().Format(time.DateOnly),
					file.Frontmatter.GetUpdatedDate().Format(time.DateOnly),
					"",
					[]string{*file.Frontmatter.Author},
					"Blog",
					nil,
				),
				postJsonLd: schemaorg.NewArticle(
					file.Frontmatter.Title+" Blog",
					[]string{path},
					nil,
					nil,
					file.Frontmatter.GetCreatedDate().Format(time.DateOnly),
					file.Frontmatter.GetUpdatedDate().Format(time.DateOnly),
					file.Frontmatter.Description,
				),
			}
			page.template = templ.Join(page.postJsonLd.ToJsonLd(), templates.Post(file))

			res.Header().
				Set("Cache-Control", "public, max-age=604800, stale-while-revalidate=604800, stale-if-error=604800")
			page.Render().ServeHTTP(res, req)
		}
	}
}

// HandlePostsFeed handles showing an RSS file for posts.
func (m *Manager) HandlePostsFeed() http.HandlerFunc {
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
			m.AppConfig.GetBaseURL().String(),
			rss.WithCopyright("Copyright 2026 Joshua Rich joshua.rich@gmail.com"),
			rss.WithManagingEditor("hello@immanent.tech (Immanent Tech)"),
			rss.WithWebmaster("hello@immanent.tech (Immanent Tech)"),
			rss.WithAtomLink(&atom.Link{
				Rel:  new(atom.LinkRelSelf),
				Href: m.AppConfig.GetBaseURL().JoinPath("/rss").String(),
				Type: new("application/rss+xml"),
			}),
			rss.WithChannelLanguage("en-us"),
			rss.WithChannelImage(&rss.Image{
				Link:  m.AppConfig.GetBaseURL().String(),
				URL:   m.AppConfig.GetBaseURL().JoinPath("/images/logo-vertical-light.webp").String(),
				Title: "Posts from the Foragd Team",
			}),
			rss.WithUpdatePeriod("monthly"),
			rss.WithUpdateFrequency(2),
		)
		for post := range slices.Values(posts) {
			contentStr, err := post.Decode("utf-8")
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Decode post content failed.",
					slog.Any("error", err))
			}
			// Generate item for post.
			item := rss.NewItem(
				rss.WithItemTitle(post.Frontmatter.Title),
				rss.WithItemDescription(post.Frontmatter.Description, false),
				rss.WithItemLink(m.AppConfig.GetBaseURL().JoinPath("/blog/"+post.Frontmatter.Slug).String()),
				rss.WithItemGUID(
					rss.NewGUID(m.AppConfig.GetBaseURL().JoinPath("/blog/"+post.Frontmatter.Slug).String(), true),
				),
				rss.WithItemImage(&types.Image{
					Title: &post.Frontmatter.Title,
					URL:   m.AppConfig.GetBaseURL().JoinPath(*post.Frontmatter.Image).String(),
				}),
				rss.WithItemContent(contentStr, true),
				rss.WithItemPublishedDate(post.Frontmatter.GetCreatedDate()),
			)
			rssFile.Channel.Items = append(rssFile.Channel.Items, *item)
		}
		slices.SortFunc(rssFile.Channel.Items, func(a rss.Item, b rss.Item) int {
			return a.GetPublishedDate().Compare(*b.GetPublishedDate())
		})
		slices.Reverse(rssFile.Channel.Items)

		// Write RSS file in response.
		res.Header().Set("Cache-Control", "public, max-age=3600, stale-while-revalidate=3600, stale-if-error=86400")
		res.Header().Set("Content-Type", rss.MimeTypes[0])
		if _, err := res.Write([]byte(xml.Header)); err != nil {
			slogctx.FromCtx(req.Context()).Error("Could not write xml header to response.",
				slog.Any("error", err),
			)
		}
		data, err := feeds.Encode(rssFile)
		if err != nil {
			slogctx.FromCtx(req.Context()).Warn("Could write RSS content to response.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
		res.Write(data)
	}
}
