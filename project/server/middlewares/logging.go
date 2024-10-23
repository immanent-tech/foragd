// Copyright (C) 2024 Joshua Rich <joshua.rich@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package middlewares

import (
	"log/slog"
	"net/http"

	slogchi "github.com/samber/slog-chi"

	"github.com/joshuar/go-feed-me/logging"
)

func Logger(logger *slog.Logger, level string, requestIDKey string) func(http.Handler) http.Handler {
	loggerConfig := slogchi.Config{
		WithRequestID: true,
	}
	slogchi.RequestIDKey = requestIDKey

	switch level {
	case "trace":
		loggerConfig.ServerErrorLevel = logging.LevelTrace
	case "debug":
		loggerConfig.ServerErrorLevel = slog.LevelDebug
	default:
		loggerConfig.ServerErrorLevel = slog.LevelInfo
	}

	return slogchi.NewWithConfig(logger.WithGroup("chi"), loggerConfig)
}
