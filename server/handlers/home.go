// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"net/http"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/web/templates"
	"github.com/joshuar/go-feed-me/web/templates/layouts"
	"github.com/joshuar/go-feed-me/web/templates/layouts/settings"
	"github.com/joshuar/go-feed-me/web/templates/partials"
	"github.com/joshuar/go-feed-me/web/views"
)

// MarkItems handles marking items as read.
func MarkItems(api DataAPI, mark models.Mark, items ...models.ItemID) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Mark the feeds.
		if resp := api.MarkItems(req.Context(), mark, items...); resp.IsError() {
			ProcessResponse(res, req, resp)
			return
		}
		res.WriteHeader(http.StatusOK)
	})
}

func GenerateHomeContent(api FeedsAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			data, resp := getHomePageData(ctx, api)
			if resp.IsError() {
				ProcessResponse(res, req, resp)
				return
			}
			articles, resp := getHomePageArticles(ctx, api, data)
			if resp.IsError() {
				ProcessResponse(res, req, resp)
				return
			}

			homePageContent := views.GenerateHomePageContent(ctx, data, articles)

			ctx = context.WithValue(ctx, contentCtxKey, homePageContent)

			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// GenerateArticle handles displaying an item as an article.
func GenerateSettings(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		settingsLayout := settings.SettingsContent()
		footer := partials.Footer(partials.FooterBackButton())

		var content []templ.Component

		if htmx.IsHTMX(req) {
			// Render content that needs updating.
			content = append(content,
				settingsLayout,
				footer,
				templates.SetPageTitle("Settings"),
			)
		} else {
			// Render a full page.
			body := templates.NewBody(settingsLayout, templates.WithBodyFooter(footer))
			page := templates.NewPage("Settings", body)
			content = append(content, page.Show())
		}

		ctx := req.Context()
		ctx = context.WithValue(ctx, templatesCtxKey, content)
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// GenerateArticle handles displaying an item as an article.
func GenerateIndex(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		indexLayout := &layouts.IndexLayout{}

		var content []templ.Component

		if htmx.IsHTMX(req) {
			// Render content that needs updating.
			content = append(content,
				indexLayout.PartialRender(),
				templates.SetPageTitle("Index"),
			)
		} else {
			// Render a full page.
			content = append(content, indexLayout.FullRender())
		}

		ctx := req.Context()
		ctx = context.WithValue(ctx, templatesCtxKey, content)
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}
