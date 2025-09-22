// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"net/http"

	"github.com/immanent-tech/foragd/providers/elastic"
)

// SetupElastic sets up handlers with the necessary data for backend
// requests with the Elastic API.
func SetupElastic() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			ctx = elastic.SetupIndexAliases(ctx)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}
