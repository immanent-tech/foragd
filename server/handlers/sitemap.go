// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"

	"github.com/indaco/teseo/schemaorg"
	slogctx "github.com/veqryn/slog-context"
)

func loadSitemapXML(baseURL *url.URL) ([]byte, error) {
	var linkMap = map[string]string{
		"Foragd Home":                   baseURL.String(),
		"About Foragd":                  baseURL.JoinPath("/about").String(),
		"Foragd Features":               baseURL.JoinPath("/features").String(),
		"Foragd Features | Collect":     baseURL.JoinPath("/features/collect").String(),
		"Foragd Features | Curate":      baseURL.JoinPath("/features/curate").String(),
		"Foragd Features | Consume":     baseURL.JoinPath("/features/consume").String(),
		"Foragd Blog":                   baseURL.JoinPath("/blog").String(),
		"Foragd Changelog":              baseURL.JoinPath("/changelog").String(),
		"Feed Viewer":                   baseURL.JoinPath("/viewer").String(),
		"Feed Linter":                   baseURL.JoinPath("/linter").String(),
		"Foragd Help":                   baseURL.JoinPath("/help").String(),
		"Compare Foragd with Feedly":    baseURL.JoinPath("/compare/feedly").String(),
		"Compare Foragd with Inoreader": baseURL.JoinPath("/compare/inoreader").String(),
		"Compare Foragd with Newsblur":  baseURL.JoinPath("/compare/newsblur").String(),
		"Compare Foragd with FreshRSS":  baseURL.JoinPath("/compare/freshrss").String(),
	}
	links := make([]schemaorg.SiteNavigationElement, 0, len(linkMap))
	var idx = 0
	for text, link := range linkMap {
		links = append(links, schemaorg.NewSimpleSiteNavigationElement(idx, text, link))
		idx++
	}
	// Add all posts.
	posts, err := getPosts()
	if err != nil {
		return nil, fmt.Errorf("generate sitemap.xml: %w", err)
	}
	for post := range slices.Values(posts) {
		links = append(
			links,
			schemaorg.NewSimpleSiteNavigationElement(
				idx,
				post.Frontmatter.Title,
				baseURL.JoinPath("/blog/"+post.Frontmatter.Slug).String(),
			),
		)
		idx++

	}

	sitemap := schemaorg.NewSiteNavigationElementList("main", links)

	data, err := sitemap.ToSitemapBytes()
	if err != nil {
		return nil, fmt.Errorf("generate sitemap.xml: %w", err)
	}
	return data, nil
}

// HandleSitemap handles requests for sitemap.xml. In the future, it may handle more requests from non natural human
// clients...
func HandleSitemap(appCfg AppConfig) http.Handler {
	sitemap, err := loadSitemapXML(appCfg.GetBaseURL())
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			slogctx.Error(r.Context(), "Cannot render sitemap.", slog.Any("error", err))
			http.Error(w, "cannot render sitemap", http.StatusInternalServerError)
			return
		})
	}
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Cache-Control", "public, max-age=86400, s-maxage=604800")
		res.Header().Set("Content-Type", "application/xml")
		res.WriteHeader(http.StatusOK)
		if _, err := res.Write(sitemap); err != nil {
			slogctx.FromCtx(req.Context()).Error("Unable to send sitemap.xml response.",
				slog.Any("error", err),
			)
		}
	})
}
