// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-syndication/rss"
	"github.com/immanent-tech/go-syndication/types"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/element"
)

type Changelog struct {
	title       templates.PageTitle `toml:"-"`
	description string              `toml:"-"`
	Releases    []templates.Release `toml:"releases"`
}

func (p *Changelog) FullResponse(res http.ResponseWriter, req *http.Request) {
	var template templ.Component

	if user := models.UserFromCtx(req.Context()); user != nil {
		template = templates.LayoutInternal(
			&templates.InternalLayoutProps{User: user},
			templates.ChangelogPage(p.Releases),
		)
	} else {
		template = templates.LayoutExternal(templates.ChangelogPage(p.Releases))
	}

	templ.Handler(
		templates.CreatePage(template,
			templates.WithPageTitle(p.title),
			templates.WithPageDescription(p.description),
			templates.WithCanonicalLink(config.GetBaseURL()+req.URL.String()),
		)).ServeHTTP(res, req)
}

func (p *Changelog) PartialResponse(res http.ResponseWriter, req *http.Request) {
	res.Header().Set(htmx.HeaderPushURL, req.URL.String())
	templ.Handler(
		templates.ChangelogPage(p.Releases),
		templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle(p.title)).ServeHTTP(res, req)
	templ.Handler(templates.SideBar(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
	templ.Handler(templates.Dock(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
}

func HandleChangelog() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		changelog := &Changelog{
			title: templates.PageTitle{
				Summary:     "Changelog",
				Description: "Latest release notes for Foragd",
			},
			description: "Latest release notes containing new features, updates and fixes for Foragd",
			Releases:    make([]templates.Release, 0),
		}

		if _, err := toml.DecodeFS(
			web.DocsFS,
			filepath.Join("assets", "docs", "changelog.toml"),
			changelog,
		); err != nil {
			slogctx.FromCtx(req.Context()).Error("Could not decode changelog toml.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusNoContent)
		}

		if user := models.UserFromCtx(req.Context()); user != nil {
			RenderInternalPage(changelog).ServeHTTP(res, req.WithContext(req.Context()))
		} else {
			RenderExternalPage(changelog).ServeHTTP(res, req.WithContext(req.Context()))
		}
	}
}

func HandleChangelogFeed() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		changelog := &Changelog{
			title: templates.PageTitle{
				Summary:     "Changelog",
				Description: "Latest release notes for Foragd",
			},
			description: "Latest release notes containing new features, updates and fixes for Foragd",
			Releases:    make([]templates.Release, 0),
		}

		if _, err := toml.DecodeFS(
			web.DocsFS,
			filepath.Join("assets", "docs", "changelog.toml"),
			changelog,
		); err != nil {
			slogctx.FromCtx(req.Context()).Error("Could not decode changelog toml.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusNoContent)
		}

		// Generate RSS file.
		rssFile := rss.NewRSS(
			changelog.title.String(),
			changelog.description,
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
			rss.WithUpdatePeriod("daily"),
		)

		for release := range slices.Values(changelog.Releases) {
			timestamp, err := time.Parse("Jan 2, 2006", release.Date)
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Unable to parse timestamp of release.",
					slog.String("release_version", release.Version),
					slog.Any("error", err),
				)
				continue
			}

			var content strings.Builder
			for change := range slices.Values(release.Changes) {
				fmt.Fprintf(&content, "%s: %s\n", change.Type, change.Description)
			}

			// Generate item for post.
			item := rss.NewItem(
				rss.WithItemTitle(release.Version),
				rss.WithItemDescription(string(release.Type)),
				rss.WithItemLink(config.GetBaseURL()+"/changelog#"+release.Version),
				rss.WithItemContent([]byte(content.String())),
				rss.WithItemPublishedDate(timestamp),
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
