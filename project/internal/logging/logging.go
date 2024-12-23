// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package logging

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
	slogmulti "github.com/samber/slog-multi"
)

const (
	LevelTrace = slog.Level(-8)
	LevelFatal = slog.Level(12)
)

var LevelNames = map[slog.Leveler]string{
	LevelTrace: "TRACE",
	LevelFatal: "FATAL",
}

var DefaultLogFile = "../deployments/server.log"

func NewLogger(level string) *slog.Logger {
	var (
		logLevel                    slog.Level
		consoleHandler, fileHandler slog.Handler
	)

	// Set the log level.
	switch level {
	case "trace":
		logLevel = LevelTrace
	case "debug":
		logLevel = slog.LevelDebug
	default:
		logLevel = slog.LevelInfo
	}

	// Set the slog handler
	consoleHandler = tint.NewHandler(os.Stderr, &tint.Options{
		Level:       logLevel,
		NoColor:     !isatty.IsTerminal(os.Stderr.Fd()),
		ReplaceAttr: levelReplacer,
	})

	// Unless no log file was requested, set up file logging.
	if DefaultLogFile != "" {
		logFile, err := openLogFile(DefaultLogFile)
		if err != nil {
			slog.Warn("Unable to open log file.",
				slog.String("file", logFile.Name()),
				slog.Any("error", err))
		} else {
			fileHandler = slog.NewJSONHandler(logFile, nil)
		}
	}

	logger := slog.New(slogmulti.Fanout(
		consoleHandler,
		fileHandler,
	))

	slog.SetDefault(logger)

	return logger
}

func levelReplacer(_ []string, attr slog.Attr) slog.Attr {
	if attr.Key == slog.LevelKey {
		level, ok := attr.Value.Any().(slog.Level)
		if !ok {
			level = slog.LevelInfo
		}

		levelLabel, exists := LevelNames[level]
		if exists {
			attr.Value = slog.StringValue(levelLabel)
		}
	}

	if err, ok := attr.Value.Any().(error); ok {
		aErr := tint.Err(err)
		attr.Key = aErr.Key
	}

	return attr
}

//nolint:mnd
func openLogFile(logFile string) (*os.File, error) {
	logFileHandle, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("unable to open log file: %w", err)
	}

	return logFileHandle, nil
}

func LogReq(req *http.Request, status int) *slog.Logger {
	return slog.Default().
		With(slog.Group("request"),
			slog.String("method", req.Method),
			slog.String("path", req.URL.Path),
			slog.Int("status", status),
		)
}

func NewHandlerLogger(handler string, req *http.Request) *slog.Logger {
	return FromContext(req.Context()).
		With(slog.Group("handler",
			slog.String("name", handler))).
		With(slog.Group("request"),
			slog.String("method", req.Method),
			slog.String("path", req.URL.Path),
		)
}
