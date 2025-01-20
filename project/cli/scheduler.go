// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:unused-receiver
package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/joshuar/go-feed-me/internal/app/scheduler"
	"github.com/joshuar/go-feed-me/internal/logging"
)

// SchedulerCmd: `go-feed-me scheduler`.
type SchedulerCmd struct {
	Migrations []string `help:"Run the scheduler."`
}

func (r *SchedulerCmd) Run(opts *CmdOpts) error {
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	ctx = logging.ToContext(ctx, opts.Logger)

	return scheduler.Run(ctx)
}
