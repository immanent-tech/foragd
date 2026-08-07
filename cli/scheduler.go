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

	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/logging"

	"github.com/immanent-tech/foragd/scheduler"
)

// SchedulerCmd defines the `scheduler` command, for performing job scheduler related actions.
type SchedulerCmd struct {
	Run       RunSchedulerCmd       `cmd:"run"        help:"Run scheduler."`
	Clear     ClearSchedulerCmd     `cmd:"clear"      help:"Clear all jobs."`
	Init      InitSchedulerCmd      `cmd:"init"       help:"Initialise the scheduler/queue."`
	ListJobs  ListJobsSchedulerCmd  `cmd:"list-jobs"  help:"List all jobs."`
	DeleteJob DeleteJobSchedulerCmd `cmd:"delete-job" help:"Job related commands."`
}

// RunSchedulerCmd is a cli command for running the scheduler component.
type RunSchedulerCmd struct{}

func (c *RunSchedulerCmd) Run() error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()
	ctx = slogctx.NewCtx(ctx, logging.New())

	// Run scheduler.
	if err := scheduler.Run(ctx); err != nil {
		return fmt.Errorf("could not run scheduler: %w", err)
	}
	return nil
}

// ClearSchedulerCmd is a cli command for clearing the scheduled jobs queue.
type ClearSchedulerCmd struct{}

func (c *ClearSchedulerCmd) Run() error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	if err := setupScheduler(ctx); err != nil {
		return fmt.Errorf("could not setup scheduler: %w", err)
	}
	// Clear job queue.
	if err := scheduler.Manager.Clear(ctx); err != nil {
		return fmt.Errorf("could not clear job queue: %w", err)
	}
	slogctx.FromCtx(ctx).Info("Job queue cleared.")
	return nil
}

// InitSchedulerCmd is a cli command to init the scheduler backend (for a new installation), without starting the
// scheduler.
type InitSchedulerCmd struct{}

func (c *InitSchedulerCmd) Run() error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	// Set up and create scheduler instance.
	if err := setupScheduler(ctx); err != nil {
		return fmt.Errorf("setup scheduler: %w", err)
	}
	// Load admin jobs.
	if err := scheduler.LoadAdminJobs(ctx); err != nil {
		return fmt.Errorf("load admin jobs: %w", err)
	}

	// Load feed update jobs.
	if err := scheduler.LoadUpdateFeedJobs(ctx); err != nil {
		return fmt.Errorf("load update feed jobs: %w", err)
	}

	slogctx.FromCtx(ctx).Info("Scheduler initialised.")
	return nil
}

// ListJobsSchedulerCmd is a cli command for listing all scheduled jobs.
type ListJobsSchedulerCmd struct{}

func (c *ListJobsSchedulerCmd) Run() error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	if err := setupScheduler(ctx); err != nil {
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

// DeleteJobSchedulerCmd is a cli command for deleting a scheduled job.
type DeleteJobSchedulerCmd struct {
	ID    string `arg:"" help:"The job ID."`
	Group string `arg:"" help:"The job group (optional)." optional:""`
}

// Run runs the delete-job scheduler command.
func (c *DeleteJobSchedulerCmd) Run() error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	if err := setupScheduler(ctx); err != nil {
		return fmt.Errorf("could not setup scheduler: %w", err)
	}

	if c.Group != "" {
		if err := scheduler.Manager.DeleteJob(
			quartz.NewJobKeyWithGroup(c.ID, c.Group),
		); err != nil {
			return fmt.Errorf("delete job: %w", err)
		}
	} else {
		if err := scheduler.Manager.DeleteJob(quartz.NewJobKey(c.ID)); err != nil {
			return fmt.Errorf("delete job: %w", err)
		}
	}

	return nil
}

func setupScheduler(ctx context.Context) error {
	ctx = slogctx.NewCtx(ctx, logging.New())
	// Run scheduler.
	if err := scheduler.NewManager(ctx); err != nil {
		return fmt.Errorf("could not run scheduler: %w", err)
	}
	return nil
}
