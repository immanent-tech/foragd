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

func DisplayArticles(dataAPI DataAPI, pagination models.Pagination, subIDs ...models.SubscriptionID) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		articles, pagination, resp := dataAPI.GetArticlesBySubscription(req.Context(), models.FiltersFromCtx(req.Context()), pagination, subIDs...)
		if resp.IsError() {
			ProcessResponse(res, req, resp)
			return
		}
		switch {
		case htmx.IsHTMX(req):
			// Update partial content for HTMX powered request.
			PartialRender(
				templ.Join(views.GenerateArticleCards(req.Context(), articles, pagination)...),
				partials.Footer(
					partials.UpdateBacklink(),
					partials.UpdateFilters(articles.GetItems().GetCategoryCounts()),
					partials.UpdateSorting(),
					partials.UpdateActions(
						views.AddSubscriptionAction(),
						views.ImportAction(),
						views.MarkAllArticlesAction(req.Context(), articles.GetSubscriptionIDs()...),
					),
				),
				templates.SetPageTitle("Items"),
			).ServeHTTP(res, req)
		default:
			// Generate full layout for non-HTMX powered request.
			layout := views.BuildArticlesLayout(req.Context(), pagination, articles)
			FullRender("Items", templates.WithBody(layout)).ServeHTTP(res, req)
		}
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
		footer := partials.Footer(partials.UpdateBacklink())
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

func DisplayHome(dataAPI DataAPI) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		subscriptionsLink := RestorePageState(req.Context(), "/home/subscriptions")
		ctx := context.WithValue(req.Context(), "subscriptionsLink", subscriptionsLink)

		content := views.GenerateHomePageContent(ctx, dataAPI.(*elastic.API))
		header := partials.Header(
			partials.DefaultHeaderStart(),
			partials.DefaultHeaderCenter(),
			partials.DefaultHeaderEnd(),
		)
		footer := partials.Footer()
		switch {
		case htmx.IsHTMX(req):
			// Update partial content for HTMX powered request.
			PartialRender(
				content,
				header,
				footer,
				templates.SetPageTitle("Home"),
			).ServeHTTP(res, req.WithContext(ctx))
		default:
			// Generate full layout for non-HTMX powered request.
			FullRender("Home", templates.WithBody(
				templates.NewBody(content,
					templates.WithBodyHeader(header),
					templates.WithBodyFooter(footer),
				),
			),
			).ServeHTTP(res, req.WithContext(ctx))
		}
	})
}
