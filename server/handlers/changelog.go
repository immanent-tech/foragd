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
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-syndication/atom"
	"github.com/immanent-tech/go-syndication/extensions/content"
	"github.com/immanent-tech/go-syndication/extensions/syndication"
	"github.com/immanent-tech/go-syndication/rss"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web"
	"github.com/immanent-tech/foragd/web/templates"
)

type Changelog struct {
	*Page

	description string              `toml:"-"`
	Releases    []templates.Release `toml:"releases"`
	link        string
}

func (m *Manager) HandleChangelog() http.HandlerFunc {
	const docPrefix = "files/docs"
	changelog := &Changelog{
		Page: m.NewPage("/changelog",
			WithSummary("Changelog"),
			WithDescription("Latest release notes for Foragd"),
			WithMetadataDescription("Latest release notes containing new features, updates and fixes for Foragd"),
		),
	}
	changelog.link = m.AppConfig.GetBaseURL().JoinPath("changelog").String()

	return func(res http.ResponseWriter, req *http.Request) {
		changelog.Releases = make([]templates.Release, 0)

		if _, err := toml.DecodeFS(
			web.Files,
			filepath.Join(docPrefix, "changelog.toml"),
			changelog,
		); err != nil {
			slogctx.FromCtx(req.Context()).Error("Could not decode changelog toml.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusNoContent)
		}

		if user := models.UserFromCtx(req.Context()); user != nil {
			changelog.template = templates.LayoutInternal(
				"",
				&templates.InternalLayoutProps{User: user},
				templates.ChangelogPage(changelog.Releases),
			)
		} else {
			changelog.template = templates.LayoutExternal(templates.ChangelogPage(changelog.Releases))
		}

		changelog.Handle().ServeHTTP(res, req)
	}
}

func (m *Manager) HandleChangelogFeed() http.HandlerFunc {
	const docPrefix = "files/docs"
	changelog := &Changelog{
		Page: m.NewPage("/changelog/feed",
			WithSummary("Changelog"),
			WithDescription("Latest release notes for Foragd"),
			WithMetadataDescription("Latest release notes containing new features, updates and fixes for Foragd"),
		),
	}

	return func(res http.ResponseWriter, req *http.Request) {
		changelog.Releases = make([]templates.Release, 0)

		if _, err := toml.DecodeFS(
			web.Files,
			filepath.Join(docPrefix, "changelog.toml"),
			changelog,
		); err != nil {
			slogctx.FromCtx(req.Context()).Error("Could not decode changelog toml.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusNoContent)
		}

		// Generate RSS file.
		rssFile := &rss.RSS{
			Channel: rss.Channel{
				Title:       changelog.GetTitle(),
				Description: changelog.GetDescription(),
				Link:        m.AppConfig.GetBaseURL().String(),
				AtomLinks: []atom.Link{
					{
						Rel:  new(atom.LinkRelSelf),
						Href: m.AppConfig.GetBaseURL().JoinPath("/changelog/feed").String(),
						Type: new("application/rss+xml"),
					},
				},
				Copyright:      new("Copyright 2026 Joshua Rich joshua.rich@gmail.com"),
				ManagingEditor: new("hello@immanent.tech (Immanent Tech)"),
				WebMaster:      new("hello@immanent.tech (Immanent Tech)"),
				Language:       new("en-US"),
				Image: &rss.Image{
					Link:  m.AppConfig.GetBaseURL().String(),
					URL:   m.AppConfig.GetBaseURL().JoinPath("/images/logo-vertical-light.webp").String(),
					Title: "Foragd Logo",
				},
				SYUdatePeriod: new(syndication.UpdatedDaily),
			},
		}

		for release := range slices.Values(changelog.Releases) {
			timestamp, err := time.Parse("Jan 2, 2006", release.Date)
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Unable to parse timestamp of release.",
					slog.String("release_version", release.Version),
					slog.Any("error", err),
				)
				continue
			}

			var contentStr strings.Builder
			for change := range slices.Values(release.Changes) {
				fmt.Fprintf(&contentStr, "%s: %s\n", change.Type, change.Description)
			}

			// Generate item for post.
			item := rss.Item{
				Title:          release.Version,
				Description:    rss.NewItemDescription(string(release.Type), false),
				Link:           m.AppConfig.GetBaseURL().JoinPath("/changelog#" + release.Version).String(),
				ContentEncoded: &content.ContentEncoded{Value: contentStr.String()},
				PubDate:        rss.NewPubDate(timestamp),
			}

			rssFile.Channel.Items = append(rssFile.Channel.Items, item)
		}
		rssFile.AutoDeclareNamespaces()

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
