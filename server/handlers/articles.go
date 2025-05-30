// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"net/http"

	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/web/views"
)

func FetchArticles(dataAPI DataAPI, pagination models.Pagination, subIDs ...models.SubscriptionID) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
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

func GenerateArticleCards(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		articles, ok := req.Context().Value(articlesCtxKey).(models.Articles)
		if !ok {
			slogctx.FromCtx(req.Context()).Warn("No subscriptions found in context.")
			next.ServeHTTP(res, req)
			return
		}
		pagination, _ := req.Context().Value(paginationCtxKey).(models.Pagination)
		templates := views.GenerateArticleCards(req.Context(), articles, pagination)
		ctx := req.Context()
		ctx = context.WithValue(ctx, templatesCtxKey, templates)
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}
