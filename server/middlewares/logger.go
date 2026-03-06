// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5/middleware"
	slogchi "github.com/immanent-tech/slog-chi"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/logging"
	gcp "github.com/immanent-tech/foragd/providers/google"
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
	slogchi.RequestGroupKey = "httpRequest"
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

		if googleCfg, err := gcp.LoadConfig(); err == nil && googleCfg.ProjectID != "" {
			traceHeader := req.Header.Get("X-Cloud-Trace-Context")
			if traceParts := strings.Split(traceHeader, "/"); len(traceParts) > 0 && len(traceParts[0]) > 0 {
				logger = logger.With(
					slog.String("trace", fmt.Sprintf("projects/%s/traces/%s", googleCfg.ProjectID, traceParts[0])),
				)
			}
		}
		// Add request ID to logger in context.
		ctx := slogctx.With(req.Context(), slog.String("id", middleware.GetReqID(req.Context())))

		// Continue handling request with updated logging.
		slogchi.NewWithConfig(logger, cfg)(next).ServeHTTP(res, req.WithContext(ctx))
	})
}
