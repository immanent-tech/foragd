// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"errors"
	"net/http"

	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/server/handlers"
)

// SetupHTMX middleware performs general setup for serving htmx-powered content.
func SetupHTMX(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Add("Vary", "HX-Request")
		next.ServeHTTP(res, req)
	})
}

// RequireHTMX middleware will only pass control to the next handler if the request is htmx powered. If not, it will
// return 403: Forbidden response.
func RequireHTMX(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if !htmx.IsHTMX(req) {
			handlers.RenderError(res, req, models.RespForbidden(errors.New("htmx is required")))
			return
		}
		next.ServeHTTP(res, req)
	})
}
