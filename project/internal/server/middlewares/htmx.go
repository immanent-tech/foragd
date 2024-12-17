// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"net/http"
	"slices"
	"strings"

	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/internal/logging"
)

// RequireHTMX checks to ensure that the given routes are using htmx. If not, an
// error is returned. Any routes not listed are assumed to not require htmx and
// passed unmodified.
func RequireHTMX(htmxRoutes []string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			if slices.ContainsFunc(htmxRoutes, func(path string) bool {
				return strings.HasPrefix(req.URL.Path, path)
			}) {
				if !htmx.IsHTMX(req) {
					logging.LogReq(req, http.StatusNotAcceptable).Error("HTMX required.")
					http.Error(res, "HTMX required.", http.StatusNotAcceptable)

					return
				}
			}

			next.ServeHTTP(res, req)
		})
	}
}
