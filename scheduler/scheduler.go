// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package scheduler contains code for the scheduler backend that handles managing background jobs for the application.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/reugn/go-quartz/logger"
	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	gerror "github.com/immanent-tech/foragd/providers/google/error"
	"github.com/immanent-tech/foragd/scheduler/jobs"
	"github.com/immanent-tech/foragd/scheduler/queue"
)

const (
	defaultOutdatedThreshold = 50 * time.Second
)

// manager contains data for managing a scheduler instance.
type manager struct {
	quartz.Scheduler

	queue quartz.JobQueue
}

var Manager *manager

// Clear will remove all jobs from the queue.
func (m *manager) Clear(ctx context.Context) error {
	if err := m.queue.Clear(); err != nil {
		return fmt.Errorf("clear job queue: %w", err)
	}
	if err := elastic.DeleteDoc(ctx, schema.SchedulerIndexRW(), "clear_deleted_feeds_state"); err != nil {
		return fmt.Errorf("clear job clear_delete_feeds state: %w", err)
	}
	if err := elastic.DeleteDoc(ctx, schema.SchedulerIndexRW(), "get_new_feeds_state"); err != nil {
		return fmt.Errorf("clear job get_new_feeds state: %w", err)
	}
	return nil
}

// Run starts the scheduler manager.
func Run(ctx context.Context) error {
	// Start the error client.
	if err := gerror.Init(); err != nil {
		return fmt.Errorf("init error client: %w", err)
	}

	if err := NewManager(ctx); err != nil {
		return fmt.Errorf("create scheduler: %w", err)
	}

	if err := RunStartupTasks(ctx); err != nil {
		return fmt.Errorf("run scheduler startup tasks: %w", err)
	}

	ctx = jobs.SchedulerAPIToCtx(ctx, Manager)

	Manager.Start(ctx)

	slogctx.FromCtx(ctx).DebugContext(ctx, "Scheduler starting.",
		slog.String("version", config.GetVersion()),
		slog.Time("start_time", time.Now()),
	)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop

	slogctx.FromCtx(ctx).DebugContext(ctx, "Scheduler stopping.",
		slog.Time("stop_time", time.Now()),
	)
	Manager.Stop()

	return nil
}

// NewManager will create a new manager object containing the scheduler and job queue.
func NewManager(ctx context.Context) error {
	// Create distributed queue instance.
	jobQueue, err := queue.NewJobQueue(ctx)
	if err != nil {
		return fmt.Errorf("new job queue: %w", err)
	}

	// Create scheduler instance.
	scheduler, err := quartz.NewStdScheduler(
		quartz.WithOutdatedThreshold(defaultOutdatedThreshold),
		quartz.WithRetryInterval(500*time.Millisecond),
		quartz.WithQueue(jobQueue, &sync.Mutex{}),
		quartz.WithLogger(logger.NewSlogLogger(ctx, slogctx.FromCtx(ctx))),
	)
	if err != nil {
		return fmt.Errorf("new scheduler: %w", err)
	}

	Manager = &manager{
		Scheduler: scheduler,
		queue:     jobQueue,
	}

	return nil
}

func LoadManager(ctx context.Context) error {
	return sync.OnceValue(func() error {
		if err := NewManager(ctx); err != nil {
			return fmt.Errorf("init scheduler: %w", err)
		}
		return nil
	})()
}

// RunStartupTasks will run a bunch of tasks that should be done when the scheduler first starts. Effectively, this
// seeds the job queue with some required jobs for scheduler functionality and maintenance.
func RunStartupTasks(ctx context.Context) error {
	ctx = jobs.SchedulerAPIToCtx(ctx, Manager)

	startupTasks, tasksCtx := errgroup.WithContext(ctx)
	defer tasksCtx.Done()

	startupTasks.Go(func() error {
		// Setup get new feeds getNewFeedsJob.
		getNewFeedsJob, err := jobs.NewGetNewFeedsJob()
		if err != nil {
			return fmt.Errorf("create new find new feeds job: %w", err)
		}
		_, err = elastic.GetDoc[string, *jobs.SerializedJob](
			ctx,
			schema.SchedulerIndexRO(),
			getNewFeedsJob.JobDetail().JobKey().String(),
		)
		if err != nil || errors.Is(err, elastic.ErrNotFound) {
			slogctx.FromCtx(ctx).Info("Adding job to find new feeds.")
			if err = Manager.ScheduleJob(getNewFeedsJob.JobDetail(), getNewFeedsJob.Trigger()); err != nil {
				return fmt.Errorf("schedule get new feeds job: %w", err)
			}
		}
		return nil
	})

	startupTasks.Go(func() error {
		// Setup clear deleted feeds job.
		clearDeletedFeedsJob, err := jobs.NewClearDeletedFeedsJob()
		if err != nil {
			return fmt.Errorf("create clear deleted feeds job: %w", err)
		}
		_, err = elastic.GetDoc[string, *jobs.SerializedJob](
			ctx,
			schema.SchedulerIndexRO(),
			clearDeletedFeedsJob.JobDetail().JobKey().String(),
		)
		if err != nil || errors.Is(err, elastic.ErrNotFound) {
			if err = Manager.ScheduleJob(clearDeletedFeedsJob.JobDetail(), clearDeletedFeedsJob.Trigger()); err != nil {
				return fmt.Errorf("schedule clear deleted feeds job: %w", err)
			}
		}
		return nil
	})

	startupTasks.Go(func() error {
		// Setup clear expired sessions job.
		clearExpiredSessionsJob, err := jobs.NewDeleteExpiredSessionsJob()
		if err != nil {
			return fmt.Errorf("create delete expired sessions job: %w", err)
		}
		_, err = elastic.GetDoc[string, *jobs.SerializedJob](
			ctx,
			schema.SchedulerIndexRO(),
			clearExpiredSessionsJob.JobDetail().JobKey().String(),
		)
		if err != nil || errors.Is(err, elastic.ErrNotFound) {
			slogctx.FromCtx(ctx).Info("Adding job to delete expired sessions.")
			if err = Manager.ScheduleJob(
				clearExpiredSessionsJob.JobDetail(),
				clearExpiredSessionsJob.Trigger(),
			); err != nil {
				return fmt.Errorf("schedule delete expired sessions job: %w", err)
			}
		}
		return nil
	})

	if err := startupTasks.Wait(); err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

	return nil
}
