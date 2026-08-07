// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package main

import (
	"errors"
	"log/slog"
	"syscall"

	"github.com/alecthomas/kong"

	"github.com/immanent-tech/go-base/logging"

	"github.com/immanent-tech/go-base/config"

	"github.com/immanent-tech/foragd/cli"
)

// CLI contains all of the commands and common options.
type CLI struct {
	Serve        cli.ServeCmd         `cmd:"" help:"Run server."`
	Elastic      cli.ElasticCmd       `cmd:"" help:"Elastic operations."`
	Scheduler    cli.SchedulerCmd     `cmd:"" help:"Run scheduler."`
	Data         cli.DataCmd          `cmd:"" help:"Manipulate data."`
	User         cli.UserCmd          `cmd:"" help:"Manipulate users."`
	Feed         cli.FeedCmd          `cmd:"" help:"Perform feed actions"`
	ProfileFlags logging.ProfileFlags `       help:"Set profiling flags." name:"profile"`
}

func main() {
	// Following is copied from https://git.kernel.org/pub/scm/libs/libcap/libcap.git/tree/goapps/web/web.go
	// ensureNotEUID aborts the program if it is running setuid something, or being invoked by root.
	if euid, uid, egid, gid := syscall.Geteuid(), syscall.Getuid(), syscall.Getegid(), syscall.Getgid(); uid != euid ||
		gid != egid ||
		uid == 0 {
		panic(errors.New("foragd should not be run with additional privileges or as root"))
	}

	commands := CLI{}
	cmd := kong.Parse(
		&commands,
		kong.Bind(),
		kong.Name(config.GetAppName()),
		kong.Description(
			"Foragd is a web-based RSS and Atom Feed Reader with a responsive design, no ads and no algorithm directing you.",
		),
		kong.UsageOnError(),
	)

	logger := logging.New()

	// Enable profiling if requested.
	if commands.ProfileFlags != nil {
		if err := logging.StartProfiling(logger, commands.ProfileFlags); err != nil {
			logger.Warn("Problem starting profiling.",
				slog.Any("error", err))
		}
	}
	// Run the requested command with the provided options.
	if err := cmd.Run(); err != nil {
		logger.Error("Command failed.",
			slog.String("command", cmd.Command()),
			slog.Any("error", err))
	}
	// If profiling was enabled, clean up.
	if commands.ProfileFlags != nil {
		if err := logging.StopProfiling(logger, commands.ProfileFlags); err != nil {
			logger.Error("Problem stopping profiling.",
				slog.Any("error", err))
		}
	}
}
