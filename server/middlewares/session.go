// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"net/http"

	"github.com/joshuar/go-feed-me/models"
)

// StoreSessionAPI stores the session API in the request context.
func StoreSessionAPI(api models.SessionAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			ctx := models.SessionToCtx(req.Context(), api)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}
