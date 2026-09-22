// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/indaco/teseo/opengraph"
	"github.com/indaco/teseo/schemaorg"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/pkg/markdownx"
	"github.com/immanent-tech/go-base/pkg/templx"
	feeds "github.com/immanent-tech/go-syndication"
	"github.com/immanent-tech/go-syndication/atom"
	"github.com/immanent-tech/go-syndication/rss"

	"github.com/immanent-tech/go-syndication/types"

	"github.com/immanent-tech/foragd/web"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/partials"
)

var getPosts = sync.OnceValues(func() ([]*markdownx.File, error) {
	var postsPath = "assets/docs/blog"
	return markdownx.ReadDir(web.DocsFS, postsPath)
})

// PostsIndex is the index of all posts.
type PostsIndex struct {
	data     templates.PostsData
	metadata pageMetadata
}

// FullResponse renders the posts index.
func (p *PostsIndex) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(templates.CreatePage(
		templates.PostsIndex(p.data),
		templates.WithPageTitle(p.metadata.Title),
		templates.WithPageDescription(p.metadata.Description),
		templates.WithCanonicalLink(p.metadata.CanonicalLink()),
		templates.WithOpenGraphMetadata(p.metadata.OpengraphData()),
		templates.WithJSONLDSchema(
			generateSiteJSONLD(p.metadata.baseURL),
			p.metadata.JSONLD(),
		),
	),
	).ServeHTTP(res, req)
}

// Post is an individual post.
type Post struct {
	*markdownx.File
	baseURL *url.URL
}

// FullResponse renders an individual post.
func (p *Post) FullResponse(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	if jsonLD, err := generatePostJSONLD(req.URL, &p.Frontmatter); err != nil {
		slogctx.Warn(ctx, "Could not generate JSON-LD for post.",
			slog.String("post", p.Frontmatter.Title),
			slog.Any("error", err),
		)
	} else {
		ctx = templx.WithSlot(
			req.Context(),
			templates.HeaderSlot,
			partials.RenderJSONLD(strings.ToLower(strings.ReplaceAll(p.Frontmatter.Title, " ", "")), jsonLD),
		)
	}
	title := templates.PageTitle{
		Summary:     p.Frontmatter.PageTitle,
		Description: "Blog",
		Date:        p.Frontmatter.GetCreatedDate().Format(time.DateOnly),
	}
	postOG := opengraph.NewArticle(
		title.String(),
		p.baseURL.Clone().JoinPath("blog", p.Frontmatter.Slug).String(),
		p.Frontmatter.Description,
		p.baseURL.Clone().JoinPath(*p.Frontmatter.Image).String(),
		p.Frontmatter.GetCreatedDate().Format(time.DateOnly),
		p.Frontmatter.GetUpdatedDate().Format(time.DateOnly),
		"",
		[]string{*p.Frontmatter.Author},
		"Blog",
		nil,
	)
	postJsonLd := schemaorg.NewArticle(
		title.String(),
		[]string{p.baseURL.Clone().JoinPath(*p.Frontmatter.Image).String()},
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
		templates.WithCanonicalLink(p.baseURL.Clone().JoinPath("blog", p.Frontmatter.Slug).String()),
		templates.WithOpenGraphMetadata(postOG),
		templates.WithJSONLDSchema(
			generateSiteJSONLD(p.baseURL),
			postJsonLd,
		),
	)).ServeHTTP(res, req.WithContext(ctx))
}

// HandlePosts handles showing the posts index or individual posts.
func HandlePosts(appCfg AppConfig) http.HandlerFunc {
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
			index := &PostsIndex{
				metadata: pageMetadata{
					Title: templates.PageTitle{
						Summary:     "Blog",
						Description: "RSS Reader Tips, Guides and Comparisons",
					},
					Description: "Guides, comparisons and tips on RSS feed readers, finding feeds, managing information overload, and taking back control of your reading from social media algorithms.",
					Path:        "/blog",
					ImagePath:   "/content/logo-vertical-light.webp",
					baseURL:     appCfg.GetBaseURL(),
				},
				data: templates.PostsData{
					Files:   posts,
					BaseURL: appCfg.GetBaseURL(),
				},
			}
			RenderExternalPage(index).ServeHTTP(res, req)
		default:
			// Individual post.
			idx := slices.IndexFunc(posts, func(p *markdownx.File) bool {
				return p.Frontmatter.Slug == slug
			})
			if idx == -1 {
				res.WriteHeader(http.StatusNotFound)
				return
			}

			res.Header().
				Set("Cache-Control", "public, max-age=604800, stale-while-revalidate=604800, stale-if-error=604800")
			RenderExternalPage(&Post{
				baseURL: appCfg.GetBaseURL(),
				File:    posts[idx],
			}).ServeHTTP(res, req)
		}
	}
}

// HandlePostsFeed handles showing an RSS file for posts.
func HandlePostsFeed(appCfg AppConfig) http.HandlerFunc {
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
			appCfg.GetBaseURL().String(),
			rss.WithCopyright("Copyright 2026 Joshua Rich joshua.rich@gmail.com"),
			rss.WithManagingEditor("hello@immanent.tech (Immanent Tech)"),
			rss.WithWebmaster("hello@immanent.tech (Immanent Tech)"),
			rss.WithAtomLink(&atom.Link{
				Rel:  new(atom.LinkRelSelf),
				Href: appCfg.GetBaseURL().JoinPath("/rss").String(),
				Type: new("application/rss+xml"),
			}),
			rss.WithChannelLanguage("en-us"),
			rss.WithChannelImage(&rss.Image{
				Link:  appCfg.GetBaseURL().String(),
				URL:   appCfg.GetBaseURL().JoinPath("/content/logo-vertical-light.webp").String(),
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
				rss.WithItemLink(appCfg.GetBaseURL().JoinPath("/blog/"+post.Frontmatter.Slug).String()),
				rss.WithItemGUID(
					rss.NewGUID(appCfg.GetBaseURL().JoinPath("/blog/"+post.Frontmatter.Slug).String(), true),
				),
				rss.WithItemImage(&types.Image{
					Title: &post.Frontmatter.Title,
					URL:   appCfg.GetBaseURL().JoinPath(*post.Frontmatter.Image).String(),
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

func generatePostJSONLD(postURL *url.URL, frontmatter *markdownx.FrontMatter) (json.RawMessage, error) {
	baseURL := postURL.Clone()
	baseURL.Path = "/"
	data := map[string]any{
		"@context":      "https://schema.org",
		"@type":         "Article",
		"headline":      frontmatter.Title,
		"description":   frontmatter.Description,
		"datePublished": frontmatter.GetCreatedDate(),
		"dateModified":  frontmatter.GetUpdatedDate(),
		"author": map[string]any{
			"@type": "Person",
			"name":  frontmatter.Author,
		},
		"publisher": map[string]any{
			"@type": "Organization",
			"name":  "Foragd",
			"url":   baseURL.String(),
		},
		"mainEntityOfPage": map[string]any{
			"@type": "WebPage",
			"@id":   postURL.String(),
		},
	}
	if frontmatter.Image != nil {
		imgURL := postURL.Clone()
		imgURL.Path = *frontmatter.Image
		data["image"] = imgURL.String()
	}

	jsonLD, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal frontmatter: %w", err)
	}
	return jsonLD, nil
}
