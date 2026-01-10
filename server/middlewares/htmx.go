// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"net/http"

	"github.com/angelofallars/htmx-go"
)

// SetupHTMX middleware performs general setup for serving htmx-powered content.
func SetupHTMX(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Add("Vary", "HX-Request")
		res.Header().Add("Vary", "HX-History-Restore-Request")
		next.ServeHTTP(res, req)
	})
}

// RequireHTMX middleware will only pass control to the next handler if the request is htmx powered. If not, it will
// return 403: Forbidden response.
func RequireHTMX(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if !htmx.IsHTMX(req) {
			http.Error(res, "Not allowed", http.StatusForbidden)
			return
		}
		next.ServeHTTP(res, req)
	})
}
