// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-feed-me/scheduler"
)

// SchedulerCmd defines the `scheduler` command, for running the job scheduler.
type SchedulerCmd struct{}

// Run contains logic for setup and execution of the scheduler.
func (r *SchedulerCmd) Run(opts *Arguments) error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()
	ctx = slogctx.NewCtx(ctx, opts.Logger)
	// Run scheduler.
	err := scheduler.Run(ctx)
	if err != nil {
		return fmt.Errorf("could not run scheduler: %w", err)
	}
	return nil
}
