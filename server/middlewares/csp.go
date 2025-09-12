// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"net/http"
)

const CSPPolicy = `base-uri 'self'; default-src 'self'; style-src 'self'; script-src 'self'; img-src *; media-src *; font-src 'self'; connect-src 'self'; frame-src *;`

// SetupCSP sets up CSP for the request.
//
// See also:
//
// https://htmx.org/docs/#csp-options
func SetupCSP() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			res.Header().Add("Content-Security-Policy", CSPPolicy)
			next.ServeHTTP(res, req)
		})
	}
}
