// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5/middleware"
	slogctx "github.com/veqryn/slog-context"
)

// AddRouteLogger adds some consistent log attributes to the logger for the given route. This can be used to correlate
// log messages coming from a particular route across any handlers the route might call.
func AddRouteLogger(ctx context.Context, route string) context.Context {
	ctx = slogctx.With(ctx, slog.String("route", route))
	ctx = slogctx.With(ctx, slog.String("id", middleware.GetReqID(ctx)))
	return ctx
}
