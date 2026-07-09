// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"encoding/xml"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/indaco/teseo/opengraph"
	"github.com/indaco/teseo/schemaorg"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-syndication/rss"

	"github.com/immanent-tech/go-syndication/types"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/pkg/formats/markdown"
	"github.com/immanent-tech/foragd/web"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/partials"
	"github.com/immanent-tech/foragd/web/templates/slots"
)

var getPosts = sync.OnceValues(func() ([]*markdown.File, error) {
	var postsPath = "assets/docs/blog"
	return markdown.ReadDir(web.DocsFS, postsPath)
})

// PostsIndex is the index of all posts.
type PostsIndex struct {
	posts []*markdown.File
}

// FullResponse renders the posts index.
func (p *PostsIndex) FullResponse(res http.ResponseWriter, req *http.Request) {
	title := templates.PageTitle{
		Summary:     "Blog",
		Description: "RSS Reader Tips, Guides and Comparisons",
	}
	description := "Guides, comparisons and tips on RSS feed readers, finding feeds, managing information overload, and taking back control of your reading from social media algorithms."
	indexOG := opengraph.NewWebSite(
		title.String(),
		config.GetBaseURL()+"/blog",
		description,
		config.GetBaseURL()+"/content/logo-vertical-light.webp",
	)
	indexJsonLd := schemaorg.NewWebPage(
		config.GetBaseURL()+"/blog",
		title.Summary,
		title.String(),
		description,
		"",
		"RSS,Atom,JSONFeed,IndieWeb",
		"en",
		config.GetBaseURL(),
		"",
		config.GetBaseURL()+"/content/logo-vertical-light.webp",
		"",
		"",
	)
	templ.Handler(templates.CreatePage(
		templates.PostsIndex(p.posts),
		templates.WithPageTitle(title),
		templates.WithPageDescription(description),
		templates.WithCanonicalLink(config.GetBaseURL()+"/blog"),
		templates.WithOpenGraphMetadata(indexOG),
		templates.WithJSONLDSchema(
			websiteJsonLd,
			indexJsonLd,
		),
	),
	).ServeHTTP(res, req)
}

// Post is an individual post.
type Post struct {
	*markdown.File
}

// FullResponse renders an individual post.
func (p *Post) FullResponse(res http.ResponseWriter, req *http.Request) {
	ctx := slots.WithSlot(
		req.Context(),
		slots.Header,
		partials.RenderJSONLD(strings.ToLower(strings.ReplaceAll(p.Frontmatter.Title, " ", "")), *p.JsonLD),
	)
	title := templates.PageTitle{
		Summary:     p.Frontmatter.PageTitle,
		Description: "Blog",
		Date:        p.Frontmatter.GetCreatedDate().Format(time.DateOnly),
	}
	postOG := opengraph.NewArticle(
		title.String(),
		config.GetBaseURL()+"/blog/"+p.Frontmatter.Slug,
		p.Frontmatter.Description,
		config.GetBaseURL()+*p.Frontmatter.Image,
		p.Frontmatter.GetCreatedDate().Format(time.DateOnly),
		p.Frontmatter.GetUpdatedDate().Format(time.DateOnly),
		"",
		[]string{*p.Frontmatter.Author},
		"Blog",
		nil,
	)
	postJsonLd := schemaorg.NewArticle(
		title.String(),
		[]string{config.GetBaseURL() + *p.Frontmatter.Image},
		nil,
		nil,
		p.Frontmatter.GetCreatedDate().Format(time.DateOnly),
		p.Frontmatter.GetUpdatedDate().Format(time.DateOnly),
		p.Frontmatter.Description,
	)
	templ.Handler(templates.CreatePage(
		templates.Post(p.File),
		templates.WithPageTitle(title),
		templates.WithPageDescription(p.Frontmatter.Description),
		templates.WithCanonicalLink(config.GetBaseURL()+"/blog/"+p.Frontmatter.Slug),
		templates.WithOpenGraphMetadata(postOG),
		templates.WithJSONLDSchema(
			websiteJsonLd,
			postJsonLd,
		),
	)).ServeHTTP(res, req.WithContext(ctx))
}

// HandlePosts handles showing the posts index or individual posts.
func HandlePosts() http.HandlerFunc {
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
		slices.SortFunc(posts, func(a, b *markdown.File) int {
			return a.Frontmatter.GetCreatedDate().Compare(b.Frontmatter.GetCreatedDate())
		})
		slices.Reverse(posts)

		// Show index when no specific post has been requested.
		switch slug := chi.URLParam(req, "*"); slug {
		case "":
			// Posts index.
			index := &PostsIndex{posts: posts}
			RenderExternalPage(index).ServeHTTP(res, req)
		default:
			// Individual post.
			idx := slices.IndexFunc(posts, func(p *markdown.File) bool {
				return p.Frontmatter.Slug == slug
			})
			if idx == -1 {
				res.WriteHeader(http.StatusNotFound)
				return
			}

			res.Header().
				Set("Cache-Control", "public, max-age=604800, stale-while-revalidate=604800, stale-if-error=604800")
			RenderExternalPage(&Post{File: posts[idx]}).ServeHTTP(res, req)
		}
	}
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
			rss.WithUpdateFrequency(2),
		)
		for post := range slices.Values(posts) {
			// Generate item for post.
			item := rss.NewItem(
				rss.WithItemTitle(post.Frontmatter.Title),
				rss.WithItemDescription(post.Frontmatter.Description),
				rss.WithItemLink(config.GetBaseURL()+"/blog/"+post.Frontmatter.Slug),
				rss.WithItemGUID(rss.GenerateGUID(config.GetBaseURL()+"/blog/"+post.Frontmatter.Slug, true)),
				rss.WithItemImage(&types.ImageInfo{
					Title: post.Frontmatter.Title,
					URL:   config.GetBaseURL() + *post.Frontmatter.Image,
				}),
				rss.WithItemContent(post.Content),
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
