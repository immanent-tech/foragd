// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package main

import (
	"embed"
	"log/slog"

	"github.com/alecthomas/kong"

	"github.com/joshuar/go-feed-me/cli"
	"github.com/joshuar/go-feed-me/internal/logging"
)

//go:embed all:static
var static embed.FS

// CLI contains all of the commands and common options.
var CLI struct {
	Serve        cli.ServeCmd         `cmd:"" help:"Run server."`
	Migrate      cli.MigrateCmd       `cmd:"" help:"Run backend migrations."`
	ProfileFlags logging.ProfileFlags `name:"profile" help:"Set profiling flags."`
	logging.Options
}

func main() {
	kong.Name("Go Feed Me")
	kong.Description("Go Feed Me handles feeds.")

	ctx := kong.Parse(&CLI, kong.Bind())

	logger := logging.New(logging.Options{LogLevel: CLI.LogLevel, NoLogFile: CLI.NoLogFile})
	// Enable profiling if requested.
	if CLI.ProfileFlags != nil {
		if err := logging.StartProfiling(CLI.ProfileFlags); err != nil {
			logger.Warn("Problem starting profiling.",
				slog.Any("error", err))
		}
	}
	// Run the requested command with the provided options.
	if err := ctx.Run(cli.AddOptions(
		cli.WithLogger(logger),
		cli.WithStaticContent(static),
	)); err != nil {
		logger.Error("Command failed.",
			slog.String("command", ctx.Command()),
			slog.Any("error", err))
	}
	// If profiling was enabled, clean up.
	if CLI.ProfileFlags != nil {
		if err := logging.StopProfiling(CLI.ProfileFlags); err != nil {
			logger.Error("Problem stopping profiling.",
				slog.Any("error", err))
		}
	}
	// // Run your server.
	// if err := runServer(); err != nil {
	// 	slog.Error("Failed to start server!", slog.Any("error", err))
	// 	os.Exit(1)
	// }
}
