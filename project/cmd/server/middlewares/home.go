// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/joshuar/go-feed-me/internal/api"
)

// CheckRequiredFilters will ensure a request for a /home route has the required
// filters set.
func CheckRequiredFilters(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if !(strings.HasPrefix(req.URL.Path, "/home") && req.Method == http.MethodGet) {
			next.ServeHTTP(res, req)
			return
		}

		ctx := req.Context()
		params := req.URL.Query()

		if !params.Has(string(api.ParamCount)) {
			params.Set(string(api.ParamCount), strconv.Itoa(api.DefaultCount))
		}

		if !params.Has(string(api.ParamView)) {
			params.Set(string(api.ParamView), string(api.DefaultView))
		}

		if !params.Has(string(api.ParamSortBy)) {
			params.Set(string(api.ParamSortBy), string(api.DefaultSortBy))
		}

		if !params.Has(string(api.ParamSortOrder)) {
			params.Set(string(api.ParamSortOrder), string(api.DefaultSortOrder))
		}

		req.URL.RawQuery = params.Encode()

		next.ServeHTTP(res, req.WithContext(ctx))
	})
}
