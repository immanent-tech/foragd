// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"log/slog"
	"net/http"
	"slices"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/config"
)

// PushCriticalAssets will optimistically send our custom script/css bundles to a client before it asks for them, which
// hopefully will speed up first page load.
func PushCriticalAssets(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {

		if pusher, ok := res.(http.Pusher); ok {
			files := []string{"scripts.js", "styles.css", "inter.css"}
			for file := range slices.Values(files) {
				route := "/content/" + file + "?v=" + config.GetVersion()
				if err := pusher.Push(route, nil); err != nil {
					slogctx.FromCtx(req.Context()).Error("Push critical asset failed.",
						slog.Group("request",
							slog.String("route", route),
							slog.String("path", req.URL.Path),
						),
						slog.Any("error", err),
					)
				}
			}
		}
		next.ServeHTTP(res, req)
	})
}
