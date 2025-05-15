// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	slogctx "github.com/veqryn/slog-context"
)

// RouteLogger decorates the logger in the request context with routing information.
func RouteLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := slogctx.With(req.Context(), slog.String("route", chi.RouteContext(req.Context()).RoutePattern()))
		ctx = slogctx.With(ctx, slog.Group("req", slog.String("id", middleware.GetReqID(ctx))))
		slogctx.FromCtx(ctx).Debug("Processing route.", slog.String("url", req.URL.String()))
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}
