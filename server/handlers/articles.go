// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"net/http"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/davecgh/go-spew/spew"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/web/templates"
	"github.com/joshuar/go-feed-me/web/templates/partials"
	"github.com/joshuar/go-feed-me/web/views"
)

func FetchArticles(dataAPI DataAPI, pagination models.Pagination, subIDs ...models.SubscriptionID) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			spew.Dump(subIDs)
			articles, pagination, resp := dataAPI.GetArticlesBySubscription(req.Context(), models.FiltersFromCtx(req.Context()), pagination, subIDs...)
			if resp.IsError() {
				ProcessResponse(res, req, resp)
				return
			}
			ctx := req.Context()
			ctx = context.WithValue(ctx, articlesCtxKey, articles)
			ctx = context.WithValue(ctx, paginationCtxKey, pagination)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

func GenerateArticleContent(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		articles, ok := req.Context().Value(articlesCtxKey).(models.Articles)
		if !ok {
			slogctx.FromCtx(req.Context()).Warn("No subscriptions found in context.")
			next.ServeHTTP(res, req)
			return
		}
		pagination, _ := req.Context().Value(paginationCtxKey).(models.Pagination)
		cards := views.GenerateArticleCards(req.Context(), articles, pagination)

		var content []templ.Component

		if req.Method == http.MethodGet {
			if !htmx.IsHTMX(req) {
				content = append(content, views.GenerateFullPageCardLayout(req.Context(), "Articles", cards...))
			} else {
				content = append(content,
					partials.LayoutCards(cards...),
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
					templates.SetPageTitle("Articles"),
				)
			}
		} else {
			content = append(content, cards...)
		}

		ctx := req.Context()
		ctx = context.WithValue(ctx, templatesCtxKey, content)
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}
