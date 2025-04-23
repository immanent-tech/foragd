// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/scheduler"
)

// ErrSchedulerCmd indicates an error occurred when running the scheduler command.
var ErrSchedulerCmd = errors.New("error running scheduler command")

// SchedulerCmd defines the `scheduler` command, for running the job scheduler.
type SchedulerCmd struct {
	Migrations []string `help:"Run the scheduler."`
}

// Run contains logic for setup and execution of the scheduler.
func (r *SchedulerCmd) Run(opts *Arguments) error {
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()
	ctx = slogctx.NewCtx(ctx, opts.Logger)

	if err := scheduler.Run(ctx); err != nil {
		return fmt.Errorf("%w: %w", ErrSchedulerCmd, err)
	}
	return nil
}
