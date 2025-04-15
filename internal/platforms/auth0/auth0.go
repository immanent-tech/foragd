// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package auth0

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5/middleware"
	slogctx "github.com/veqryn/slog-context"
)

func logger(ctx context.Context) *slog.Logger {
	logger := slogctx.FromCtx(ctx)
	if id := middleware.GetReqID(ctx); id != "" {
		logger = logger.With(slog.String("id", id))
	}
	return logger.WithGroup("auth0")
}
