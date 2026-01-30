// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"log/slog"
	"net/http"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/config"
)

// SetCacheControl sets an appropriate Cache-Control header for user content based on the user's update frequency
// setting.
func SetCacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Cache-Control", "private, max-age=0, must-revalidate")
		next.ServeHTTP(res, req)
	})
}

// PushCriticalAssets will optimistically send our custom script/css bundles to a client before it asks for them, which
// hopefully will speed up first page load.
func PushCriticalAssets(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if pusher, ok := res.(http.Pusher); ok {
			if err := pusher.Push("/content/scripts.js?v="+config.Version, nil); err != nil {
				slogctx.FromCtx(req.Context()).Error("Push scripts failed.",
					slog.Any("error", err),
				)
			}
			if err := pusher.Push("/content/styles.css?v="+config.Version, nil); err != nil {
				slogctx.FromCtx(req.Context()).Error("Push styles failed.",
					slog.Any("error", err),
				)
			}
			if err := pusher.Push("/content/inter.css?v="+config.Version, nil); err != nil {
				slogctx.FromCtx(req.Context()).Error("Push styles failed.",
					slog.Any("error", err),
				)
			}
		}
		next.ServeHTTP(res, req)
	})
}
