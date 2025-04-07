// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package cli

import (
	"embed"
	"log/slog"
)

type CmdOpts struct {
	Logger        *slog.Logger
	StaticContent embed.FS
}

type Option func(*CmdOpts) *CmdOpts

func AddOptions(options ...Option) *CmdOpts {
	commandOptions := &CmdOpts{}
	for _, option := range options {
		option(commandOptions)
	}

	return commandOptions
}

func WithStaticContent(content embed.FS) Option {
	return func(ctx *CmdOpts) *CmdOpts {
		ctx.StaticContent = content
		return ctx
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(ctx *CmdOpts) *CmdOpts {
		ctx.Logger = logger
		return ctx
	}
}
