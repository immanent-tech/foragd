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
	"github.com/joshuar/go-feed-me/web/templates"
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

// DisplayArticle handles displaying an item as an article.
func DisplayArticle(dataAPI DataAPI, itemID models.ItemID) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		article, found, resp := dataAPI.GetArticle(req.Context(), itemID)
		if resp.IsError() || !found {
			ProcessResponse(res, req, resp)
			return
		}
		content := views.BuildArticleLayout(article)
		header := partials.Header(
			partials.DefaultHeaderStart(),
			partials.DefaultHeaderCenter(),
			partials.DefaultHeaderEnd(),
		)
		footer := partials.Footer(partials.BackButton())
		switch {
		case htmx.IsHTMX(req):
			// Update partial content for HTMX powered request.
			PartialRender(
				content,
				footer,
				templates.SetPageTitle(article.Item.GetTitle()),
			).ServeHTTP(res, req)
		default:
			// Generate full layout for non-HTMX powered request.
			FullRender(article.Item.GetTitle(),
				templates.WithBody(
					templates.NewBody(views.BuildArticleLayout(article),
						templates.WithBodyHeader(header),
						templates.WithBodyFooter(footer),
					),
				),
			).ServeHTTP(res, req)
		}
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
					templates.SetPageTitle("Home"),
				)
			default:
				body := templates.NewBody(homePageContent,
					templates.WithBodyHeader(partials.DefaultHeader()),
					templates.WithBodyFooter(partials.Footer()),
				)
				page := templates.NewPage("Home", templates.WithBody(body))
				content = append(content, page.Template())
			}
			ctx = context.WithValue(ctx, templatesCtxKey, content)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}
