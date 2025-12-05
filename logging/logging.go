// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package logging

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/fatih/color"
	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
	slogmulti "github.com/samber/slog-multi"
	slogctx "github.com/veqryn/slog-context"
	slogjson "github.com/veqryn/slog-json"
)

const (
	// LevelTrace is a custom TRACE log level.
	LevelTrace = slog.Level(-8)
	// LevelFatal is a custom FATAL log level.
	LevelFatal = slog.Level(12)
)

// LevelNames contains a list of custom log level names.
var LevelNames = map[slog.Leveler]string{
	LevelTrace: "TRACE",
	LevelFatal: "FATAL",
}

// Options are options for controlling logging.
type Options struct {
	LogLevel  string `env:"FORAGD_LOGLEVEL"  name:"log-level"   enum:"info,debug,trace" default:"info"  help:"Set logging level."`
	NoLogFile bool   `env:"FORAGD_NOLOGFILE" name:"no-log-file"                         default:"false" help:"Don't write to a log file."`
}

// DefaultLogFile is the default log file location.
var (
	DefaultLogFile = "/app/foragd.log"
	Level          slog.Level
)

// New creates a new logger with the given options.
func New(options Options) *slog.Logger {
	var (
		logFile  string
		handlers []slog.Handler
	)

	// Set the log level.
	switch options.LogLevel {
	case "trace":
		Level = LevelTrace
	case "debug":
		Level = slog.LevelDebug
	default:
		Level = slog.LevelInfo
	}

	// Set a log file if specified.
	if options.NoLogFile {
		logFile = ""
	} else {
		logFile = DefaultLogFile
	}

	// When logging in a conainer, use json output and disable log file, otherwise, use colourful output.
	if os.Getenv("FORAGD_CONTAINER") == "1" {
		logFile = ""
		handlers = append(handlers,
			slogjson.NewHandler(os.Stderr, containerConsoleOptions(Level)),
		)
	} else {
		handlers = append(handlers,
			tint.NewHandler(os.Stderr, consoleOptions(Level, os.Stderr.Fd())),
		)
	}

	// Unless no log file was requested, set up file logging.
	if logFile != "" {
		if logFH, err := openLogFile(logFile); err != nil {
			fmt.Fprintln(os.Stderr, "unable to open log file: %w", err)
		} else {
			handlers = append(handlers,
				slogjson.NewHandler(logFH, generateFileOpts(Level)),
			)
		}
	}

	logger := slog.New(slogctx.NewHandler(slogmulti.Fanout(handlers...), nil))
	slog.SetDefault(logger)

	return logger
}

func containerConsoleOptions(level slog.Level) *slogjson.HandlerOptions {
	opts := &slogjson.HandlerOptions{
		AddSource:   false,
		Level:       level,
		ReplaceAttr: fileLevelReplacer,
		JSONOptions: json.JoinOptions(
			json.Deterministic(true),
			jsontext.EscapeForJS(false),
			jsontext.EscapeForHTML(true),
			jsontext.SpaceAfterColon(true),
			jsontext.SpaceAfterComma(true),
		),
	}
	if level == LevelTrace {
		opts.AddSource = true
	}
	return opts
}

func consoleOptions(level slog.Level, fd uintptr) *tint.Options {
	opts := &tint.Options{
		Level:       level,
		NoColor:     !isatty.IsTerminal(fd),
		ReplaceAttr: consolelevelReplacer,
		TimeFormat:  time.Kitchen,
	}
	if level == LevelTrace {
		opts.AddSource = true
	}

	return opts
}

func generateFileOpts(level slog.Level) *slogjson.HandlerOptions {
	opts := &slogjson.HandlerOptions{
		AddSource:   false,
		Level:       level,
		ReplaceAttr: fileLevelReplacer,
	}
	if level == LevelTrace {
		opts.AddSource = true
	}

	return opts
}

func consolelevelReplacer(_ []string, attr slog.Attr) slog.Attr {
	if attr.Key == slog.LevelKey {
		level, ok := attr.Value.Any().(slog.Level)
		if !ok {
			level = slog.LevelInfo
		}
		switch level {
		case slog.LevelError:
			attr.Value = slog.StringValue(color.HiRedString("ERROR"))
		case slog.LevelWarn:
			attr.Value = slog.StringValue(color.HiYellowString("WARN"))
		case slog.LevelInfo:
			attr.Value = slog.StringValue(color.HiGreenString("INFO"))
		case slog.LevelDebug:
			attr.Value = slog.StringValue(color.HiMagentaString("DEBUG"))
		case LevelTrace:
			attr.Value = slog.StringValue(color.HiWhiteString("TRACE"))
		default:
			attr.Value = slog.StringValue("UNKNOWN")
		}
	}

	return attr
}

func fileLevelReplacer(_ []string, attr slog.Attr) slog.Attr {
	// Set default level.
	if attr.Key == slog.LevelKey {
		level, ok := attr.Value.Any().(slog.Level)
		if !ok {
			level = slog.LevelInfo
		}

		// Format custom log level.
		if levelLabel, exists := LevelNames[level]; exists {
			attr.Value = slog.StringValue(levelLabel)
		}
	}

	return attr
}

// openLogFile will attempt to open the specified log file. It will also attempt
// to create the directory containing the log file if it does not exist.
func openLogFile(logFile string) (*os.File, error) {
	logDir := filepath.Dir(logFile)
	// Create the log directory if it does not exist.
	if _, err := os.Stat(logDir); err == nil || errors.Is(err, os.ErrNotExist) {
		err = os.MkdirAll(logDir, 0o750)
		if err != nil {
			return nil, fmt.Errorf("unable to create log file directory %s: %w", logDir, err)
		}
	}

	// Open the log file.
	logFileHandle, err := os.Create(logFile) // #nosec:G304
	if err != nil {
		return nil, fmt.Errorf("unable to open log file: %w", err)
	}

	return logFileHandle, nil
}
