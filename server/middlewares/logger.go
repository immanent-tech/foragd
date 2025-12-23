// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"log/slog"
	"net/http"
	"sync"

	slogchi "github.com/samber/slog-chi"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/logging"
)

var configureLogging = sync.OnceValue(func() slogchi.Config {
	cfg := slogchi.Config{
		ClientErrorLevel: slog.LevelWarn,
		ServerErrorLevel: slog.LevelError,
		WithRequestID:    true,
		Filters: []slogchi.Filter{
			slogchi.IgnorePathContains("/content", "/favicon"),
		},
	}
	switch logging.Level {
	case logging.LevelTrace:
		cfg.WithRequestBody = true
		cfg.WithResponseBody = true
		cfg.WithRequestHeader = true
		cfg.WithResponseHeader = true
	}
	return cfg
})

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		cfg := configureLogging()

		slogchi.NewWithConfig(slogctx.FromCtx(req.Context()), cfg)(next).ServeHTTP(res, req)
	})
}
