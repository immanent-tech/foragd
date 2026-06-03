// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-syndication/rss"
	"github.com/immanent-tech/go-syndication/types"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/element"
)

var releases = []templates.Release{
	{
		Version:  "v0.153",
		Date:     "June 3, 2026",
		Type:     templates.VersionMajor,
		IsLatest: true,
		Changes: []templates.ChangeEntry{
			{
				Type:        templates.ChangeTypeNew,
				Description: "Added a changelog page.",
			},
		},
	},
	{
		Version: "v0.152",
		Date:    "June 3, 2026",
		Type:    templates.VersionPatch,
		Changes: []templates.ChangeEntry{
			{
				Type:        templates.ChangeTypeImproved,
				Description: "Foragd now suggests a colour for the browser or OS to use when rendering surfaces around the site based on your theme.",
			},
			{
				Type:        templates.ChangeTypeImproved,
				Description: "OPML export has been streamlined.",
			},
			{
				Type:        templates.ChangeTypeImproved,
				Description: "The informed/inspired/enlightened starter feedsets have been updated and now generated dynamically.",
			},
		},
	},
	{
		Version: "v0.151",
		Date:    "Jun 2, 2026",
		Type:    templates.VersionPatch,
		Changes: []templates.ChangeEntry{
			{
				Type:        templates.ChangeTypeFixed,
				Description: "You can now focus the global search box with Alt+k again.",
			},
			{
				Type:        templates.ChangeTypeImproved,
				Description: "Foragd will try to protect you from breaching your account limits.",
			},
		},
	},
	{
		Version: "v0.150",
		Date:    "May 29, 2026",
		Type:    templates.VersionMajor,
		Changes: []templates.ChangeEntry{
			{
				Type:        templates.ChangeTypeNew,
				Description: "There is now a dedicated page for bulk and individual management of all your subscriptions.",
			},
		},
	},
}

type Changelog struct {
	title    string
	template templ.Component
}

// FullResponse renders a full page (headers, footers and list of subscriptions).
func (p *Changelog) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(p.template,
			templates.WithPageTitle(p.title),
			templates.WithCanonicalLink(config.GetBaseURL()+req.URL.String()),
		)).ServeHTTP(res, req)
}

func (p *Changelog) PartialResponse(res http.ResponseWriter, req *http.Request) {
	res.Header().Set(htmx.HeaderPushURL, req.URL.String())
	templ.Handler(p.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle(p.title)).ServeHTTP(res, req)
	templ.Handler(templates.SideBar(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
	templ.Handler(templates.Dock(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
}

func HandleChangelog() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		if user := models.UserFromCtx(req.Context()); user != nil {
			RenderInternalPage(&Help{
				template: templates.LayoutInternal(
					&templates.InternalLayoutProps{User: user},
					templates.ChangelogPage(releases),
				),
			}).ServeHTTP(res, req.WithContext(req.Context()))
		} else {
			RenderExternalPage(&Help{
				template: templates.LayoutExternal(
					templates.ChangelogPage(releases),
				),
			}).ServeHTTP(res, req.WithContext(req.Context()))
		}
	}
}

func HandleChangelogFeed() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Generate RSS file.
		rssFile := rss.NewRSS(
			"Foragd Changelog",
			"Foragd updates and improvements — shipped often",
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

		for release := range slices.Values(releases) {
			timestamp, err := time.Parse("January 2, 2006", release.Date)
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
