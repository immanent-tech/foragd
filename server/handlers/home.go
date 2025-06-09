// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"net/http"

	"github.com/joshuar/go-feed-me/web/views"
)

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
			ctx = context.WithValue(ctx, titleCtxKey, "Go Feed Me Home")

			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}
