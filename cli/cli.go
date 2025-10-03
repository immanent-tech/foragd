// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package cli contains functionality for the command-line interface of the service. It provides commands for starting
// both the server and scheduler services.
package cli

import (
	"embed"
	"log/slog"
)

// Arguments are the common options for commands.
type Arguments struct {
	// Logger is a logger that the command can use.
	Logger *slog.Logger
	// StaticContent is an embedded filesystem containing static files.
	StaticContent embed.FS
	// Environment is the running environment for the application.
	Environment string
}

// Option is a functional option for the command-line.
type Option func(*Arguments) *Arguments

// AddArguments generates command-line arguments from the given options.
func AddArguments(options ...Option) *Arguments {
	commandOptions := &Arguments{}
	for _, option := range options {
		option(commandOptions)
	}

	return commandOptions
}

// WithLogger defines a logger for a command.
func WithLogger(logger *slog.Logger) Option {
	return func(ctx *Arguments) *Arguments {
		ctx.Logger = logger
		return ctx
	}
}

// WithEnvironment option sets the environment for the command.
func WithEnvironment(env string) Option {
	return func(ctx *Arguments) *Arguments {
		ctx.Environment = env
		return ctx
	}
}
