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
	"slices"
	"sync"
	"time"

	"github.com/reugn/go-quartz/logger"
	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/scheduler/jobs"
	"github.com/immanent-tech/foragd/scheduler/queue"
)

const (
	defaultOutdatedThreshold = 50 * time.Second
)

// manager contains data for managing a scheduler instance.
type manager struct {
	quartz.Scheduler

	queue          quartz.JobQueue
	misfiredJobsCh chan quartz.ScheduledJob
}

var Manager *manager

// Clear will remove all jobs from the queue.
func (m *manager) Clear(ctx context.Context) error {
	if err := m.queue.Clear(); err != nil {
		return fmt.Errorf("clear job queue: %w", err)
	}
	return nil
}

// Run starts the scheduler manager.
func Run(ctx context.Context) error {
	if err := NewManager(ctx); err != nil {
		return fmt.Errorf("create scheduler: %w", err)
	}

	// Create an indexer that jobs can use and store it in the context for access by jobs.
	indexer, err := bulk.NewIndexer(ctx, bulk.WithFlushInterval(time.Minute, 5*time.Second))
	if err != nil {
		return fmt.Errorf("create indexer: %w", err)
	}
	ctx = jobs.IndexerToCtx(ctx, indexer)

	// Store the scheduler in the context for access by jobs.
	ctx = jobs.SchedulerAPIToCtx(ctx, Manager)

	// Start a goroutine to process misfired jobs.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case job := <-Manager.misfiredJobsCh:
				slogctx.FromCtx(ctx).Warn("Running misfired job.",
					slog.String("job_key", job.JobDetail().JobKey().String()),
				)
				// Immediately run misfired job.
				if err := job.JobDetail().Job().Execute(ctx); err != nil {
					slogctx.FromCtx(ctx).Error("Misfired job failed.",
						slog.String("job_key", job.JobDetail().JobKey().String()),
						slog.Any("error", err),
					)
				}
			}
		}
	}()

	// Load all admin jobs as needed.
	if err := LoadAdminJobs(ctx); err != nil {
		return fmt.Errorf("run scheduler startup tasks: %w", err)
	}

	// Start scheduling jobs.
	Manager.Start(ctx)
	slogctx.FromCtx(ctx).DebugContext(ctx, "Scheduler starting.",
		slog.String("version", config.GetVersion()),
		slog.Time("start_time", time.Now()),
	)

	// Wait until we get a signal to stop.
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

	misfiredJobsCh := make(chan quartz.ScheduledJob)

	// Create scheduler instance.
	scheduler, err := quartz.NewStdScheduler(
		quartz.WithOutdatedThreshold(defaultOutdatedThreshold),
		quartz.WithRetryInterval(500*time.Millisecond),
		quartz.WithQueue(jobQueue, &sync.Mutex{}),
		quartz.WithLogger(logger.NewSlogLogger(ctx, slogctx.FromCtx(ctx))),
		quartz.WithMisfiredChan(misfiredJobsCh),
	)
	if err != nil {
		return fmt.Errorf("new scheduler: %w", err)
	}

	Manager = &manager{
		Scheduler:      scheduler,
		queue:          jobQueue,
		misfiredJobsCh: misfiredJobsCh,
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

// LoadAdminJobs will run a bunch of tasks that should be done when the scheduler first starts. Effectively, this
// seeds the job queue with some required jobs for scheduler functionality and maintenance.
func LoadAdminJobs(ctx context.Context) error {
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

func LoadUpdateFeedJobs(ctx context.Context) error {
	// Get all feeds.
	feeds, err := elastic.SearchAll[*models.Feed](ctx, schema.FeedsIndexRO(), query.MatchAll(), 5000)
	if err != nil {
		return fmt.Errorf("get feeds: %w", err)
	}

	for feed := range slices.Values(feeds) {
		// Add additional feed details to logs.
		feedCtx := slogctx.With(ctx, "feed_id", feed.GetID())
		feedCtx = slogctx.With(feedCtx, "feed_name", feed.GetTitle())

		// Create a job for the feed.
		jobKey := quartz.NewJobKeyWithGroup(feed.GetID(), "update_feed")
		// Check if there is an existing scheduled job.
		switch existingJob, err := Manager.GetScheduledJob(jobKey); {
		case err != nil && models.HTTPStatus(err) != http.StatusNotFound && !errors.Is(err, quartz.ErrJobNotFound):
			// If we cannot ascertain if there is an existing scheduled job, skip this feed.
			slogctx.FromCtx(feedCtx).Warn("Unable to check for existing scheduled job.",
				slog.String("feed_id", feed.GetID()),
				slog.Any("error", err),
			)
		case errors.Is(err, quartz.ErrJobNotFound):
			// If there is no existing scheduled newJob, create one.
			newJob, err := jobs.NewUpdateFeedJob(ctx, feed.GetID())
			if err != nil {
				slogctx.FromCtx(feedCtx).Warn("Unable to create new update feed job for feed.",
					slog.Any("error", err),
				)
				continue
			}

			// Schedule the new job.
			if err = Manager.ScheduleJob(newJob.JobDetail(), newJob.Trigger()); err != nil {
				slogctx.FromCtx(feedCtx).Error("Failed to schedule new job for feed.",
					slog.String("job_id", newJob.JobDetail().JobKey().String()),
					slog.String("job_schedule", newJob.Trigger().Description()),
					slog.Any("error", err),
				)
				continue
			}
			slogctx.FromCtx(feedCtx).Debug("Added new job for feed.",
				slog.String("job_id", newJob.JobDetail().JobKey().String()),
				slog.String("job_schedule", newJob.Trigger().Description()),
			)
			// // Do an initial run of the job.
			// if err = newJob.JobDetail().Job().Execute(ctx); err != nil {
			// 	slogctx.FromCtx(feedCtx).Error("Failed initial run of update feed job.",
			// 		slog.String("job_id", newJob.JobDetail().JobKey().String()),
			// 		slog.String("job_schedule", newJob.Trigger().Description()),
			// 		slog.Any("error", err),
			// 	)
			// }
		case existingJob != nil:
			// Existing job found, ignore.
			slogctx.FromCtx(feedCtx).Debug("Existing job found, ignoring.",
				slog.String("job_id", existingJob.JobDetail().JobKey().String()),
				slog.String("feed_id", feed.GetID()),
			)
		default:
			// Unhandled result.
			slogctx.FromCtx(feedCtx).Debug("Unhandled result.",
				slog.String("feed_id", feed.GetID()),
			)
		}
	}

	return nil
}
