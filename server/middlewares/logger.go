// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"log/slog"
	"net/http"
	"sync"

	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5/middleware"
	slogchi "github.com/samber/slog-chi"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/logging"
)

var configureLogging = sync.OnceValue(func() slogchi.Config {
	cfg := slogchi.Config{
		ClientErrorLevel: slog.LevelWarn,
		ServerErrorLevel: slog.LevelError,
		WithSpanID:       true,
		WithTraceID:      true,
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

		logger := slogctx.FromCtx(req.Context())
		if htmx.IsHTMX(req) {
			// Decorate logger with HTMX specific attributes.
			logger = logger.With(
				slog.Group("request",
					slog.Group("htmx",
						slog.Bool("is_htmx", htmx.IsHTMX(req)),
						slog.Bool("is_history_restore_request", htmx.IsHistoryRestoreRequest(req)),
						slog.String("target", req.Header.Get(htmx.HeaderTarget)),
						slog.String("trigger", req.Header.Get(htmx.HeaderTrigger)),
					),
				),
			)
		}
		// Add request ID to logger in context.
		ctx := slogctx.With(req.Context(), slog.String("id", middleware.GetReqID(req.Context())))

		// Continue handling request with updated logging.
		slogchi.NewWithConfig(logger, cfg)(next).ServeHTTP(res, req.WithContext(ctx))
	})
}
