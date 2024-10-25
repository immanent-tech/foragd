// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package logging

import (
	"context"
	"errors"
	"log/slog"
)

type contextKey string

const (
	loggerCtxKey contextKey = "logger"
)

var ErrNotInCtx = errors.New("config not found in context")

func ToContext(ctx context.Context, logger *slog.Logger) context.Context {
	newCtx := context.WithValue(ctx, loggerCtxKey, logger)

	return newCtx
}

func FromContext(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value(loggerCtxKey).(*slog.Logger)
	if !ok {
		return slog.Default()
	}

	return logger
}
