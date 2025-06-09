// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"net/http"
	"strings"
)

// SetupCSP sets up CSP for the request.
func SetupCSP(csp []string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			res.Header().Add("Content-Security-Policy", strings.Join(csp, " "))
			next.ServeHTTP(res, req)
		})
	}
}
