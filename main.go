// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package main

import (
	"embed"
	"log/slog"
	"os"

	"github.com/alecthomas/kong"

	"github.com/joshuar/go-feed-me/cli"
	"github.com/joshuar/go-feed-me/config"
	"github.com/joshuar/go-feed-me/logging"
)

//go:embed all:static
var static embed.FS

// CLI contains all of the commands and common options.
var CLI struct {
	Serve        cli.ServeCmd         `cmd:"" help:"Run server."`
	Migrate      cli.MigrateCmd       `cmd:"" help:"Run backend migrations."`
	Scheduler    cli.SchedulerCmd     `cmd:"" help:"Run scheduler."`
	Prune        cli.PruneCmd         `cmd:"" help:"Run pruning."`
	ProfileFlags logging.ProfileFlags `name:"profile" help:"Set profiling flags."`
	logging.Options
}

func main() {
	kong.Name("Go Feed Me")
	kong.Description("Go Feed Me handles feeds.")

	ctx := kong.Parse(&CLI, kong.Bind())

	logger := logging.New(logging.Options{LogLevel: CLI.LogLevel, NoLogFile: CLI.NoLogFile})

	if err := config.Init(); err != nil {
		logger.Error("Could not initialize config.",
			slog.Any("error", err))
		os.Exit(-1)
	}

	// Enable profiling if requested.
	if CLI.ProfileFlags != nil {
		if err := logging.StartProfiling(CLI.ProfileFlags); err != nil {
			logger.Warn("Problem starting profiling.",
				slog.Any("error", err))
		}
	}
	// Run the requested command with the provided options.
	if err := ctx.Run(cli.AddArguments(
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
}
