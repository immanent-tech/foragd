/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

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
	"github.com/immanent-tech/foragd/service"
)

// SchedulerCmd defines the `scheduler` command, for performing job scheduler related actions.
type SchedulerCmd struct {
	Run       RunSchedulerCmd       `cmd:"" help:"Run scheduler."`
	Clear     ClearSchedulerCmd     `cmd:"" help:"Clear all jobs."`
	Init      InitSchedulerCmd      `cmd:"" help:"Initialise the scheduler/queue."`
	ListJobs  ListJobsSchedulerCmd  `cmd:"" help:"List all jobs."`
	DeleteJob DeleteJobSchedulerCmd `cmd:"" help:"Job related commands."`
}

// RunSchedulerCmd is a cli command for running the scheduler component.
type RunSchedulerCmd struct{}

func (c *RunSchedulerCmd) Run() error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()
	ctx = slogctx.NewCtx(ctx, logging.New())

	manager, err := scheduler.NewManager(ctx)
	if err != nil {
		return fmt.Errorf("new scheduler: %w", err)
	}

	if err := manager.Run(ctx); err != nil {
		return fmt.Errorf("run scheduler: %w", err)
	}
	return nil
}

// ClearSchedulerCmd is a cli command for clearing the scheduled jobs queue.
type ClearSchedulerCmd struct{}

func (c *ClearSchedulerCmd) Run() error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	manager, err := scheduler.NewManager(ctx)
	if err != nil {
		return fmt.Errorf("new scheduler: %w", err)
	}

	// Clear job queue.
	if err := manager.Clear(); err != nil {
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

	feedSvc, err := service.LoadFeedService()
	if err != nil {
		return fmt.Errorf("load feed service: %w", err)
	}

	// Set up and create scheduler instance.
	manager, err := scheduler.NewManager(ctx)
	if err != nil {
		return fmt.Errorf("new scheduler: %w", err)
	}
	// Load admin jobs.
	if err := manager.InitAdminJobs(ctx); err != nil {
		return fmt.Errorf("load admin jobs: %w", err)
	}

	// Load feed update jobs.
	if err := manager.LoadUpdateFeedJobs(ctx, feedSvc); err != nil {
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

	manager, err := scheduler.NewManager(ctx)
	if err != nil {
		return fmt.Errorf("new scheduler: %w", err)
	}

	// Clear job queue.
	keys, err := manager.GetJobKeys()
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

	manager, err := scheduler.NewManager(ctx)
	if err != nil {
		return fmt.Errorf("new scheduler: %w", err)
	}

	if c.Group != "" {
		if err := manager.DeleteJob(
			quartz.NewJobKeyWithGroup(c.ID, c.Group),
		); err != nil {
			return fmt.Errorf("delete job: %w", err)
		}
	} else {
		if err := manager.DeleteJob(quartz.NewJobKey(c.ID)); err != nil {
			return fmt.Errorf("delete job: %w", err)
		}
	}

	return nil
}
