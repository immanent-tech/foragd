// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/config"

	"github.com/immanent-tech/foragd/web/templates"
)

// StorePaths stores the current request path and the value of the "from" request parameter (else any local referrer
// path).
func StorePaths(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Store "from" path if set, or use referrer header.
		from := req.URL.Query().Get("from")
		if from == "" {
			refURL, err := url.Parse(req.Referer())
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Could not parse referer.", slog.Any("error", err))
				from = "/home"
			}
			if strings.HasPrefix(refURL.String(), config.GetBaseURL()) {
				from = refURL.Path
			}
		}
		if !isSafeLocalPath(from) {
			from = "/home"
		}
		ctx := templates.FromPathToCtx(req.Context(), from)
		ctx = templates.PathToCtx(ctx, req.URL.Path)
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

func isSafeLocalPath(p string) bool {
	return strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "//")
}
