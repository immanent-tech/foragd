// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:unused-receiver
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/joshuar/go-feed-me/internal/app/scheduler"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/server"
)

// SchedulerCmd: `go-feed-me scheduler`.
type SchedulerCmd struct {
	Migrations []string `help:"Run the scheduler."`
}

func (r *SchedulerCmd) Run(opts *CmdOpts) error {
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	ctx = logging.ToContext(ctx, opts.Logger)

	// Load the config.
	if err := server.LoadConfig(); err != nil {
		return fmt.Errorf("unable to load server config: %w", err)
	}

	return scheduler.Run(ctx, server.Environment())
}
