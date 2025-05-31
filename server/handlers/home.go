// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"net/http"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
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

func GenerateHomeContent(api DataAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			subscriptionsLink := RestorePageState(req.Context(), "/home/subscriptions")
			ctx := context.WithValue(req.Context(), "subscriptionsLink", subscriptionsLink)
			homePageContent := views.GenerateHomePageContent(ctx, api.(*elastic.API))

			var content []templ.Component

			switch {
			case htmx.IsHTMX(req):
				content = append(content,
					homePageContent,
					partials.Footer(),
					partials.SetPageTitle("Home"),
				)
			default:
				body := partials.NewBody(homePageContent,
					partials.WithBodyFooter(partials.Footer()),
				)
				page := partials.NewPage("Home", partials.WithBody(body))
				content = append(content, page.Template())
			}
			ctx = context.WithValue(ctx, templatesCtxKey, content)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// GenerateArticle handles displaying an item as an article.
func GenerateSettings(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		settingsLayout := settings.SettingsContent()

		var content []templ.Component

		if !htmx.IsHTMX(req) {
			body := partials.NewBody(settingsLayout, partials.WithBodyFooter(partials.BackButton()))
			content = append(content, partials.NewPage("Settings", partials.WithBody(body)).Template())
		} else {
			// Append content that needs updating.
			content = append(content,
				settingsLayout,
				partials.Footer(partials.BackButton()),
				partials.SetPageTitle("Settings"),
			)
		}

		ctx := req.Context()
		ctx = context.WithValue(ctx, templatesCtxKey, content)
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}
