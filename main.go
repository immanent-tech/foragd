// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package main

import (
	"context"
	"embed"
	"log/slog"
	"os"
	"syscall"

	"github.com/alecthomas/kong"
	slogelasticsearch "github.com/immanent-tech/slog-elasticsearch"

	"github.com/immanent-tech/foragd/cli"
	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/logging"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/schema"
)

//go:embed all:web/content
var static embed.FS

// CLI contains all of the commands and common options.
var CLI struct {
	logging.Options

	Serve        cli.ServeCmd         `cmd:"" help:"Run server."`
	Migrate      cli.MigrateCmd       `cmd:"" help:"Run backend migrations."`
	Scheduler    cli.SchedulerCmd     `cmd:"" help:"Run scheduler."`
	ProfileFlags logging.ProfileFlags `name:"profile" help:"Set profiling flags."`
}

func init() {
	// Following is copied from https://git.kernel.org/pub/scm/libs/libcap/libcap.git/tree/goapps/web/web.go
	// ensureNotEUID aborts the program if it is running setuid something,
	// or being invoked by root.
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

	ctx := kong.Parse(&CLI, kong.Bind())

	if err := config.Init(); err != nil {
		slog.Error("Could not initialize config.",
			slog.Any("error", err))
		os.Exit(-1)
	}

	// Load the Elastic backend
	esapi, err := elastic.RawConnection(context.Background())
	if err != nil {
		slog.Error("Could not initialize config.",
			slog.Any("error", err))
		os.Exit(-1)
	}
	esLogHandler := slogelasticsearch.Option{Level: slog.LevelDebug, Conn: esapi, Index: schema.LogsSchemaPrefix}.NewElasticsearchHandler(context.Background())

	logger := logging.New(logging.Options{LogLevel: CLI.LogLevel, NoLogFile: CLI.NoLogFile, Handlers: []slog.Handler{esLogHandler}})

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
