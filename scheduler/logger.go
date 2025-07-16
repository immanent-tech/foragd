// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package scheduler

import "log/slog"

type logger struct {
	*slog.Logger
}

func (l *logger) Trace(msg string, args ...any) {
	l.Debug(msg, args...)
}
