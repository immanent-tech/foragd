// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"net/http"

	"github.com/joshuar/go-feed-me/components/config"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/providers/elastic/schema"
)

// ElasticMiddleware sets up handlers with the necessary data for backend
// requests with the Elastic API.
func ElasticMiddleware() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			ctx = elastic.FeedsIndexToCtx(ctx, schema.FeedsSchemaPrefix)
			ctx = elastic.ItemsIndexToCtx(ctx, schema.FeedItemsSchemaPrefix+"_"+config.Environment())
			ctx = elastic.UserIndexToCtx(ctx, schema.UsersSchemaPrefix)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}
