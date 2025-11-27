// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"log/slog"
	"net/http"

	slogchi "github.com/samber/slog-chi"

	"github.com/immanent-tech/foragd/logging"
)

func Logger() func(http.Handler) http.Handler {
	cfg := slogchi.Config{
		ClientErrorLevel: slog.LevelWarn,
		ServerErrorLevel: slog.LevelError,
		WithRequestID:    true,
		Filters: []slogchi.Filter{
			slogchi.IgnorePathContains("/content", "/favicon"),
		},
	}
	switch logging.Level { //nolint:gocritic // leaving for future expansion.
	case logging.LevelTrace:
		cfg.WithRequestBody = true
		cfg.WithResponseBody = true
		cfg.WithRequestHeader = true
		cfg.WithResponseHeader = true
	}
	return slogchi.NewWithConfig(slog.Default(), cfg)
}
