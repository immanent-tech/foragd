// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"net/http"

	"github.com/joshuar/go-feed-me/internal/config"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/schema"
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
			logging.FromContext(ctx).Debug("Loaded index patterns to context.")
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}
