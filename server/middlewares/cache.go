// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"net/http"
)

// NoCache ensures no caching will be applied.
func NoCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Cache-Control", "private, no-cache, max-age=0")
		next.ServeHTTP(res, req)
	})
}
