// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sync"

	"github.com/indaco/teseo/schemaorg"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/config"
)

var loadSitemapXML = sync.OnceValues(func() ([]byte, error) {
	var linkMap = map[string]string{
		"Foragd Home":                   config.GetBaseURL(),
		"About Foragd":                  config.GetBaseURL() + "/about",
		"Foragd Features":               config.GetBaseURL() + "/features",
		"Foragd Features | Collect":     config.GetBaseURL() + "/features/collect",
		"Foragd Features | Curate":      config.GetBaseURL() + "/features/curate",
		"Foragd Features | Consume":     config.GetBaseURL() + "/features/consume",
		"Foragd Blog":                   config.GetBaseURL() + "/blog",
		"Foragd Changelog":              config.GetBaseURL() + "/changelog",
		"Feed Viewer":                   config.GetBaseURL() + "/viewer",
		"Feed Linter":                   config.GetBaseURL() + "/linter",
		"Foragd Help":                   config.GetBaseURL() + "/help",
		"Compare Foragd with Feedly":    config.GetBaseURL() + "/compare/feedly",
		"Compare Foragd with Inoreader": config.GetBaseURL() + "/compare/inoreader",
		"Compare Foragd with Newsblur":  config.GetBaseURL() + "/compare/newsblur",
		"Compare Foragd with FreshRSS":  config.GetBaseURL() + "/compare/freshrss",
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
				config.GetBaseURL()+"/blog/"+post.Frontmatter.Slug,
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
})

// HandleSitemap handles requests for sitemap.xml. In the future, it may handle more requests from non natural human
// clients...
func HandleSitemap() http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		sitemap, err := loadSitemapXML()
		if err != nil {
			http.NotFound(res, req)
			return
		}
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
