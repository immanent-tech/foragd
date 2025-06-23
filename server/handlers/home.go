// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"net/http"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/web/views"
)

// GenerateHomeContent handles generating the content for the home page.
func GenerateHomeContent(api models.DocumentsAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			ctx = context.WithValue(ctx, titleCtxKey, "Go Feed Me Home")

			subscriptionsLink := RestorePageState(ctx, "/home/subscriptions")
			articlesLink := RestorePageState(ctx, "/home/articles")

			data, resp := views.NewHomePageData(ctx, api, subscriptionsLink, articlesLink)
			if resp.IsNotFound() {
				ctx = context.WithValue(ctx, contentCtxKey, views.EmptyContent())
				next.ServeHTTP(res, req.WithContext(ctx))
				return
			}
			if resp != nil {
				ProcessResponse(res, req, resp)
				return
			}

			ctx = context.WithValue(ctx, contentCtxKey, data.Show())

			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}
