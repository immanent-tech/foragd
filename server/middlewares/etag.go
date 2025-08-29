// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"net/http"

	"github.com/go-http-utils/etag"
)

// Etag calculates and adds an appropriate e-tag header to the response.
//
// https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/ETag
func Etag(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if ct := req.Header.Get("Accept"); ct == "text/event-stream" {
			// Don't use etags for SSE/eventstream responses.
			next.ServeHTTP(res, req)
		} else {
			etag.Handler(next, false).ServeHTTP(res, req)
		}
	})
}
