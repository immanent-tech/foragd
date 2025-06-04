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
	"github.com/joshuar/go-feed-me/web/templates/partials"
	"github.com/joshuar/go-feed-me/web/views"
)

func GenerateArticleCollection(api FeedsAPI, pagination models.Pagination, subIDs ...models.SubscriptionID) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			articles, pagination, resp := getFilteredArticles(req.Context(), api, pagination, subIDs...)
			if resp.IsError() {
				ProcessResponse(res, req, resp)
				return
			}

			cards := views.GenerateArticleCards(req.Context(), articles, pagination)

			cardLayout := partials.CardGrid(cards...)
			cardControls := partials.CardControls(
				views.RefreshAction(),
				views.UpdateSorting(),
				views.UpdateFilters(articles.GetItems().GetCategoryCounts()),
				views.CollectionActionsMenu(
					views.MarkAllArticlesAction(req.Context(), articles.GetSubscriptionIDs()...),
				),
			)

			ctx := context.WithValue(req.Context(), contentCtxKey, templ.Join(cardControls, cardLayout))
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// GenerateArticle handles displaying an item as an article.
func GenerateArticle(dataAPI DataAPI, itemID models.ItemID) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			article, found, resp := dataAPI.GetArticle(req.Context(), itemID)
			if resp.IsError() || !found {
				ProcessResponse(res, req, resp)
				return
			}
			articleLayout := views.BuildArticleLayout(article)
			footer := partials.Footer(partials.FooterBackButton())

			var content []templ.Component

			if htmx.IsHTMX(req) {
				// Append content that needs updating.
				content = append(content, articleLayout, footer, templates.SetPageTitle(article.Item.GetTitle()))
			} else {
				body := templates.NewBody(articleLayout, templates.WithBodyFooter(footer))
				page := templates.NewPage(article.Item.GetTitle(), body)
				content = append(content, page.Show())
			}

			ctx := req.Context()
			ctx = context.WithValue(ctx, templatesCtxKey, content)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}
