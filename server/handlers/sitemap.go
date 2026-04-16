// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sync"

	"github.com/immanent-tech/go-syndication/sitemap"
	slogctx "github.com/veqryn/slog-context"
)

//nolint:mnd // thes are individual page priorities.
var loadSitemapXML = sync.OnceValues(func() ([]byte, error) {
	// Set up default URLs.
	site := sitemap.NewURLSet(
		sitemap.URL{
			Loc:      "https://foragd.app",
			Priority: 1.0,
		},
		sitemap.URL{
			Loc:      "https://foragd.app/features",
			Priority: 1.0,
		},
		sitemap.URL{
			Loc:      "https://foragd.app/viewer",
			Priority: 0.9,
		},
		sitemap.URL{
			Loc:      "https://foragd.app/blog",
			Priority: 0.9,
		},
		sitemap.URL{
			Loc:      "https://foragd.app/about",
			Priority: 0.75,
		},
		sitemap.URL{
			Loc:      "https://foragd.app/help",
			Priority: 0.6,
		},
		sitemap.URL{
			Loc:      "https://foragd.app/compare/feedly",
			Priority: 0.8,
		},
		sitemap.URL{
			Loc:      "https://foragd.app/compare/inoreader",
			Priority: 0.8,
		},
	)
	// Add all posts.
	posts, err := getPosts()
	if err != nil {
		return nil, fmt.Errorf("generate sitemap.xml: %w", err)
	}
	for post := range slices.Values(posts.Files) {
		site.URLs = append(site.URLs,
			sitemap.URL{
				Loc:      sitemap.LOC("https://foragd.app/blog/" + post.Path),
				Priority: 0.5,
			})
	}

	data, err := xml.Marshal(site)
	if err != nil {
		return nil, fmt.Errorf("generate sitemap.xml: %w", err)
	}
	return []byte(xml.Header + string(data)), nil
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
		res.Header().Set("Cache-Control", "public, max-age=604800, s-maxage=43200, must-revalidate")
		res.Header().Set("Content-Type", "application/xml")
		res.WriteHeader(http.StatusOK)
		if _, err := res.Write(sitemap); err != nil {
			slogctx.FromCtx(req.Context()).Error("Unable to send sitemap.xml response.",
				slog.Any("error", err),
			)
		}
	})
}
