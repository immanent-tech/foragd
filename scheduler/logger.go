// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package scheduler

import (
	"log/slog"

	"github.com/joshuar/go-feed-me/logging"
)

type logger struct {
	*slog.Logger
}

func (l *logger) Trace(msg string, args ...any) {
	if logging.Level == logging.LevelTrace {
		l.Debug(msg, args...)
	}
}
