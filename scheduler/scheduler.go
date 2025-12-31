// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package scheduler contains code for the scheduler backend that handles managing background jobs for the application.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/schema"
	"github.com/immanent-tech/foragd/scheduler/jobs"
	"github.com/immanent-tech/foragd/scheduler/queue"
)

const (
	defaultOutdatedThreshold = 50 * time.Second
)

var ErrScheduler = errors.New("scheduler encountered an error")

// manager contains data for managing a scheduler instance.
type manager struct {
	quartz.Scheduler

	queue quartz.JobQueue
}

var Manager *manager

// Run starts the scheduler manager.
func Run(ctx context.Context) error {
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
		// quartz.WithLogger(logger),
	)
	if err != nil {
		return fmt.Errorf("new scheduler: %w", err)
	}

	Manager = &manager{
		Scheduler: scheduler,
		queue:     jobQueue,
	}

	ctx = jobs.SchedulerAPIToCtx(ctx, Manager)

	// Setup get new feeds job.
	job, err := jobs.NewGetNewFeedsJob()
	if err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}
	_, err = Manager.GetScheduledJob(job.JobDetail().JobKey())
	if err != nil && errors.Is(err, quartz.ErrJobNotFound) {
		err = Manager.ScheduleJob(job.JobDetail(), job.Trigger())
		if err != nil {
			return fmt.Errorf("failed to start scheduler: %w", err)
		}
	}

	// Check for new feeds on startup.
	err = job.JobDetail().Job().Execute(ctx)
	if err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

	scheduler.Start(ctx)

	slogctx.FromCtx(ctx).DebugContext(ctx, "Scheduler started.")

	var wg sync.WaitGroup
	wg.Add(1)
	// Listen for shutdown events and process them.
	go func() {
		wg.Done()
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt)
		<-stop
		// Can't do much here except for logging any errors
		if err != nil {
			slog.Error("Error occurred when trying to shut down server.",
				slog.Any("error", err),
			)
		}
	}()
	if errors.Is(err, http.ErrServerClosed) { // graceful shutdown
		wg.Wait()
	} else if err != nil {
		return fmt.Errorf("error shutting down server: %w", err)
	}

	<-ctx.Done()
	scheduler.Stop()
	slogctx.FromCtx(ctx).DebugContext(ctx, "Scheduler stopped.")
	return nil
}

// GetJobState returns the job state of the job with the given id.
func (m *manager) GetJobState(ctx context.Context, id string) (*models.JobState, error) {
	state, err := elastic.GetDoc[string, *models.JobState](ctx, schema.SchedulerIndexRO, id)
	if err != nil {
		return nil, fmt.Errorf("scheduler: get job state: %w", err)
	}
	return state, nil
}

// UpdateJobState will update the job state for the job with the given id.
func (m *manager) UpdateJobState(ctx context.Context, id string, updates map[string]any) error {
	updates["updated_at"] = time.Now().UTC()
	if err := elastic.UpdateDoc(ctx, schema.SchedulerIndexRW, id, updates,
		elastic.UpdateDocAsUpsert(),
		elastic.WithRefresh("true"),
	); err != nil {
		return fmt.Errorf("scheduled: update job state: %w", err)
	}
	return nil
}
