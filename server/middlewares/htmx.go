// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import "net/http"

// SetupHTMX middleware performs general setup for serving htmx-powered content.
func SetupHTMX() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			res.Header().Add("Vary", "HX-Request")
			next.ServeHTTP(res, req)
		})
	}
}
