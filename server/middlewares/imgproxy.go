// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"net/http"
	"strings"
)

func ProxyImage() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			s, _, _ := strings.Cut(req.Header.Get("Content-Type"), ";")
			s = strings.ToLower(strings.TrimSpace(s))
			next.ServeHTTP(res, req)
		})
	}
}
