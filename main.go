// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//nolint:sloglint
package main

import (
	"log/slog"
	"os"
	"syscall"

	"github.com/alecthomas/kong"

	"github.com/immanent-tech/foragd/cli"
	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/logging"
)

// CLI contains all of the commands and common options.
var CLI struct {
	logging.Options

	Serve        cli.ServeCmd         `cmd:"" help:"Run server."`
	Schema       cli.SchemaCmd        `cmd:"" help:"Perform schema operations."`
	Scheduler    cli.SchedulerCmd     `cmd:"" help:"Run scheduler."`
	Data         cli.DataCmd          `cmd:"" help:"Manipulate data."`
	User         cli.UserCmd          `cmd:"" help:"Manipulate users."`
	Feed         cli.FeedCmd          `cmd:"" help:"Perform feed actions"`
	ProfileFlags logging.ProfileFlags `name:"profile" help:"Set profiling flags."`
	Environment  string               `env:"FORAGD_ENVIRONMENT" name:"environment" help:"Set the running environment." required:"true" default:"development" enum:"development,production"`
}

func init() {
	// Following is copied from https://git.kernel.org/pub/scm/libs/libcap/libcap.git/tree/goapps/web/web.go
	// ensureNotEUID aborts the program if it is running setuid something, or being invoked by root.
	euid := syscall.Geteuid()
	uid := syscall.Getuid()
	egid := syscall.Getegid()
	gid := syscall.Getgid()

	if uid != euid || gid != egid || uid == 0 {
		slog.Error("foragd should not be run with additional privileges or as root.")
		os.Exit(-1)
	}
}

func main() {
	kong.Name(config.AppName)
	kong.Description(config.AppDescription)

	cmd := kong.Parse(&CLI, kong.Bind())

	if err := config.Init(); err != nil {
		slog.Error("Could not initialize config.",
			slog.Any("error", err))
		os.Exit(-1)
	}

	logger := logging.New(logging.Options{LogLevel: CLI.LogLevel, NoLogFile: CLI.NoLogFile})

	// Enable profiling if requested.
	if CLI.ProfileFlags != nil {
		if err := logging.StartProfiling(logger, CLI.ProfileFlags); err != nil {
			logger.Warn("Problem starting profiling.",
				slog.Any("error", err))
		}
	}
	// Run the requested command with the provided options.
	if err := cmd.Run(cli.AddArguments(
		cli.WithLogger(logger),
		cli.WithEnvironment(CLI.Environment),
	)); err != nil {
		logger.Error("Command failed.",
			slog.String("command", cmd.Command()),
			slog.Any("error", err))
	}
	// If profiling was enabled, clean up.
	if CLI.ProfileFlags != nil {
		if err := logging.StopProfiling(logger, CLI.ProfileFlags); err != nil {
			logger.Error("Problem stopping profiling.",
				slog.Any("error", err))
		}
	}
}
