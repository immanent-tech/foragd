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

	"github.com/immanent-tech/foragd/config"
)

//nolint:mnd // thes are individual page priorities.
var loadSitemapXML = sync.OnceValues(func() ([]byte, error) {
	links := make([]schemaorg.SiteNavigationElement, 0)
	links = append(
		links,
		schemaorg.NewSimpleSiteNavigationElement(1, "Home", config.GetBaseURL()),
		schemaorg.NewSimpleSiteNavigationElement(2, "About", config.GetBaseURL()+"/about"),
		schemaorg.NewSimpleSiteNavigationElement(3, "Features", config.GetBaseURL()+"/features"),
		schemaorg.NewSimpleSiteNavigationElement(4, "Features | Collect", config.GetBaseURL()+"/features/collect"),
		schemaorg.NewSimpleSiteNavigationElement(5, "Features | Curate", config.GetBaseURL()+"/features/curate"),
		schemaorg.NewSimpleSiteNavigationElement(6, "Features | Consume", config.GetBaseURL()+"/features/consume"),
		schemaorg.NewSimpleSiteNavigationElement(7, "Blog", config.GetBaseURL()+"/blog"),
		schemaorg.NewSimpleSiteNavigationElement(8, "Changelog", config.GetBaseURL()+"/changelog"),
		schemaorg.NewSimpleSiteNavigationElement(10, "Viewer", config.GetBaseURL()+"/viewer"),
		schemaorg.NewSimpleSiteNavigationElement(11, "Help", config.GetBaseURL()+"/help"),
		schemaorg.NewSimpleSiteNavigationElement(12, "Compare with Feedly", config.GetBaseURL()+"/compare/feedly"),
		schemaorg.NewSimpleSiteNavigationElement(
			13,
			"Compare with Inoreader",
			config.GetBaseURL()+"/compare/inoreader",
		),
	)
	// Add all posts.
	posts, err := getPosts()
	if err != nil {
		return nil, fmt.Errorf("generate sitemap.xml: %w", err)
	}
	i := 13
	for post := range slices.Values(posts.Files) {
		links = append(
			links,
			schemaorg.NewSimpleSiteNavigationElement(i, post.File, config.GetBaseURL()+"/"+post.Path),
		)
		i++

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
