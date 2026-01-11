// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package scheduler contains code for the scheduler backend that handles managing background jobs for the application.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/reugn/go-quartz/logger"
	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
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

// GetJobState returns the job state of the job with the given id.
func (m *manager) GetJobState(ctx context.Context, id string) (*models.JobState, error) {
	state, err := elastic.GetDoc[string, *models.JobState](ctx, models.SchedulerIndexRO, id)
	if err != nil {
		return nil, fmt.Errorf("scheduler: get job state: %w", err)
	}
	return state, nil
}

// UpdateJobState will update the job state for the job with the given id.
func (m *manager) UpdateJobState(ctx context.Context, id string, updates map[string]any) error {
	updates["updated_at"] = time.Now().UTC()
	if err := elastic.UpdateDoc(ctx, models.SchedulerIndexRW, id, updates,
		elastic.UpdateDocAsUpsert(),
		elastic.WithRefresh("true"),
	); err != nil {
		return fmt.Errorf("scheduled: update job state: %w", err)
	}
	return nil
}

// Clear will remove all jobs from the queue.
func (m *manager) Clear() error {
	if err := m.queue.Clear(); err != nil {
		return fmt.Errorf("clear job queue: %w")
	}
	return nil
}

// Run starts the scheduler manager.
func Run(ctx context.Context) error {
	if err := NewManager(ctx); err != nil {
		return fmt.Errorf("create scheduler: %w", err)
	}

	if err := RunStartupTasks(ctx); err != nil {
		return fmt.Errorf("run scheduler startup tasks: %w", err)
	}

	Manager.Start(ctx)

	slogctx.FromCtx(ctx).DebugContext(ctx, "Scheduler starting.")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop

	slogctx.FromCtx(ctx).DebugContext(ctx, "Scheduler stopping.")
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
	// logger := &logger{Logger: slogctx.FromCtx(ctx)}
	scheduler, err := quartz.NewStdScheduler(
		quartz.WithOutdatedThreshold(defaultOutdatedThreshold),
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
			return fmt.Errorf("create get new feeds job: %w", err)
		}
		_, err = Manager.GetScheduledJob(getNewFeedsJob.JobDetail().JobKey())
		if err != nil && errors.Is(err, quartz.ErrJobNotFound) {
			err = Manager.ScheduleJob(getNewFeedsJob.JobDetail(), getNewFeedsJob.Trigger())
			if err != nil {
				return fmt.Errorf("check get new feeds job: %w", err)
			}
		}
		// Check for new feeds on startup.
		if err := getNewFeedsJob.JobDetail().Job().Execute(ctx); err != nil {
			return fmt.Errorf("schedule get new feeds job: %w", err)
		}
		return nil
	})

	startupTasks.Go(func() error {
		// Setup clear deleted feeds job.
		clearDeletedFeedsJob, err := jobs.NewClearDeletedFeedsJob()
		if err != nil {
			return fmt.Errorf("create clear deleted feeds job: %w", err)
		}
		_, err = Manager.GetScheduledJob(clearDeletedFeedsJob.JobDetail().JobKey())
		if err != nil && errors.Is(err, quartz.ErrJobNotFound) {
			err = Manager.ScheduleJob(clearDeletedFeedsJob.JobDetail(), clearDeletedFeedsJob.Trigger())
			if err != nil {
				return fmt.Errorf("check clear deleted feeds job: %w", err)
			}
		}
		// Check for new feeds on startup.
		if err = clearDeletedFeedsJob.JobDetail().Job().Execute(ctx); err != nil {
			return fmt.Errorf("schedule clear deleted feeds job: %w", err)
		}
		return nil
	})

	if err := startupTasks.Wait(); err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

	return nil
}
