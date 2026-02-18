// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"syscall"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/scheduler"
)

// SchedulerCmd defines the `scheduler` command, for running the job scheduler.
type SchedulerCmd struct {
	Run   RunSchedulerCmd   `cmd:"run"   help:"Run scheduler."`
	Clear ClearSchedulerCmd `cmd:"clear" help:"Clear all jobs."`
	Init  InitSchedulerCmd  `cmd:"init"  help:"Initialise the scheduler/queue."`
	List  ListSchedulerCmd  `cmd:"list"  help:"List all jobs."`
}

type RunSchedulerCmd struct{}
type ClearSchedulerCmd struct{}
type InitSchedulerCmd struct{}
type ListSchedulerCmd struct{}

// Run contains logic for setup and execution of the scheduler.
func (c *RunSchedulerCmd) Run(opts *Arguments) error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()
	ctx = slogctx.NewCtx(ctx, opts.Logger)
	// Run scheduler.
	if err := scheduler.Run(ctx); err != nil {
		return fmt.Errorf("could not run scheduler: %w", err)
	}
	return nil
}

// Run runs the clear command that will remove all scheduled jobs.
func (c *ClearSchedulerCmd) Run(opts *Arguments) error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	if err := setupScheduler(ctx, opts); err != nil {
		return fmt.Errorf("could not setup scheduler: %w", err)
	}
	// Clear job queue.
	if err := scheduler.Manager.Clear(ctx); err != nil {
		return fmt.Errorf("could not clear job queue: %w", err)
	}
	slogctx.FromCtx(ctx).Info("Job queue cleared.")
	return nil
}

// Run runs the clear command that will remove all scheduled jobs.
func (c *InitSchedulerCmd) Run(opts *Arguments) error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	if err := setupScheduler(ctx, opts); err != nil {
		return fmt.Errorf("could not setup scheduler: %w", err)
	}
	// Clear job queue.
	if err := scheduler.RunStartupTasks(ctx); err != nil {
		return fmt.Errorf("could not init scheduler: %w", err)
	}
	slogctx.FromCtx(ctx).Info("Scheduler initialised.")
	return nil
}

// Run runs the clear command that will remove all scheduled jobs.
func (c *ListSchedulerCmd) Run(opts *Arguments) error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	if err := setupScheduler(ctx, opts); err != nil {
		return fmt.Errorf("could not setup scheduler: %w", err)
	}

	// Clear job queue.
	keys, err := scheduler.Manager.GetJobKeys()
	if err != nil {
		return fmt.Errorf("could not list jobs: %w", err)
	}

	for key := range slices.Values(keys) {
		fmt.Println(key.String())
	}

	return nil
}

func setupScheduler(ctx context.Context, opts *Arguments) error {
	ctx = slogctx.NewCtx(ctx, opts.Logger)
	// Run scheduler.
	if err := scheduler.NewManager(ctx); err != nil {
		return fmt.Errorf("could not run scheduler: %w", err)
	}
	return nil
}
