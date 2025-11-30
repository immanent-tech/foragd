// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"net/http"
	"strings"
)

var CSPPolicy = []string{
	`base-uri 'self';`,
	`default-src 'self';`,
	`style-src 'self' 'unsafe-inline';`,
	`script-src 'self' 'unsafe-eval' 'unsafe-inline' https://cdn.jsdelivr.net https://cloud.umami.is https://static.cloudflareinsights.com;`,
	`img-src * data: *;`,
	`media-src *;`,
	`font-src 'self';`,
	`connect-src 'self' https://cdn.jsdelivr.net https://cloud.umami.is https://static.cloudflareinsights.com;`,
	`frame-src *;`,
}

// SetupCSP sets up CSP for the request.
//
// See also:
//
// https://htmx.org/docs/#csp-options
func SetupCSP() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			res.Header().Add("Content-Security-Policy", strings.Join(CSPPolicy, " "))
			next.ServeHTTP(res, req)
		})
	}
}
