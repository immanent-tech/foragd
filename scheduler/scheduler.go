/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

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
	"strings"
	"sync"
	"time"

	"github.com/reugn/go-quartz/logger"
	"github.com/reugn/go-quartz/matcher"
	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/go-base/client"
	"github.com/immanent-tech/go-base/config"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/scheduler/jobs"
	"github.com/immanent-tech/foragd/scheduler/queue"
	"github.com/immanent-tech/foragd/server/cache"
	"github.com/immanent-tech/foragd/service"
)

const (
	defaultOutdatedThreshold = 50 * time.Second
	gracefulShutdownTimeout  = 30 * time.Second
)

// manager contains data for managing a scheduler instance.
type manager struct {
	quartz.Scheduler

	queue quartz.JobQueue
	store *service.ElasticService
}

var Manager *manager

// Clear will remove all jobs from the queue.
func (m *manager) Clear(ctx context.Context) error {
	if err := m.queue.Clear(); err != nil {
		return fmt.Errorf("clear job queue: %w", err)
	}
	return nil
}

func (m *manager) UpdateSerializedJob(ctx context.Context, job *jobs.SerializedJob) error {
	if err := bulk.AddAction(ctx,
		bulk.NewAction(
			job,
			bulk.AsOperation[string](bulk.OpIndex),
			bulk.ToIndex[string](m.store.GetIndexRW(service.ScheduleIndex)),
		),
	); err != nil {
		return fmt.Errorf("update serialized job: %w", err)
	}
	if err := bulk.Flush(ctx); err != nil {
		slogctx.Warn(ctx, "Unable to flush bulk request.",
			slog.Any("error", err))
	}
	return nil
}

// Run starts the scheduler manager.
func Run(ctx context.Context) error {
	appCfg, err := config.LoadAppConfig()
	if err != nil {
		return fmt.Errorf("load app config: %w", err)
	}

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

	ctx = jobs.ElasticToCtx(ctx, Manager.store)

	// Load the http client.
	httpClient, err := client.Load()
	if err != nil {
		return fmt.Errorf("load http client: %w", err)
	}
	httpClient = httpClient.SetHeader(
		"User-Agent",
		appCfg.GetAppName()+"/"+appCfg.GetAppVersion()+" (+https://foragd.app/policies/bot)",
	)
	ctx = jobs.HTTPClientToCtx(ctx, httpClient)

	// Load the articles cache.
	itemsCache, err := cache.NewItemsCache()
	if err != nil {
		return fmt.Errorf("load items cache: %w", err)
	}
	ctx = jobs.ItemCacheToCtx(ctx, itemsCache)

	userSvc, err := service.LoadUserService()
	if err != nil {
		return fmt.Errorf("load user service: %w", err)
	}
	ctx = jobs.UserSvcToCtx(ctx, userSvc)

	feedSvc, err := service.LoadFeedService()
	if err != nil {
		return fmt.Errorf("load feed service: %w", err)
	}
	ctx = jobs.FeedSvcToCtx(ctx, feedSvc)

	itemSvc, err := service.LoadItemService()
	if err != nil {
		return fmt.Errorf("load item service: %w", err)
	}
	ctx = jobs.ItemSvcToCtx(ctx, itemSvc)

	// Load all admin jobs as needed.
	if err := InitAdminJobs(ctx); err != nil {
		return fmt.Errorf("run scheduler startup tasks: %w", err)
	}

	// Start scheduling jobs.
	Manager.Start(ctx)
	slogctx.Info(ctx, "Scheduler started.",
		slog.String("version", appCfg.Version),
		slog.Time("start_time", time.Now()),
	)

	// Wait until we get a signal to stop.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	// Create shutdown context with 30-second timeout
	shutdownCtx, cancel := context.WithTimeoutCause(
		context.Background(),
		gracefulShutdownTimeout,
		errors.New("graceful shutdown timeout"),
	)
	defer cancel()

	if err := bulk.Shutdown(shutdownCtx); err != nil {
		slogctx.FromCtx(shutdownCtx).Error("Failed to shut down indexer gracefully.",
			slog.Any("error", err))
	}
	if err := elastic.Shutdown(shutdownCtx); err != nil {
		slogctx.FromCtx(shutdownCtx).Error("Elasticsearch failed to shutdown gracefully.",
			slog.Any("error", err),
		)
	}
	Manager.Stop()

	slogctx.FromCtx(shutdownCtx).Debug("Scheduler stopped.",
		slog.Time("stop_time", time.Now()),
	)

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

	// Load the elastic service.
	elasticSvc, err := service.LoadElasticService()
	if err != nil {
		return fmt.Errorf("load elastic service: %w", err)
	}

	Manager = &manager{
		Scheduler: scheduler,
		queue:     jobQueue,
		store:     elasticSvc,
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

// InitAdminJobs loads the listed jobs into the scheduler. These are administrative jobs that should always be
// scheduled.
func InitAdminJobs(ctx context.Context) error {
	ctx = jobs.SchedulerAPIToCtx(ctx, Manager)

	// List of jobs to activate at startup.
	var startupJobs = []func() (*jobs.SerializedJob, error){
		jobs.NewGetNewFeedsJob,
		jobs.NewClearDeletedFeedsJob,
		jobs.NewDeleteExpiredSessionsJob,
		jobs.NewRestartFeedUpdatesJob,
	}

	startupTasks, tasksCtx := errgroup.WithContext(ctx)
	defer tasksCtx.Done()

	for job := range slices.Values(startupJobs) {
		startupTasks.Go(func() error {
			serialized, err := job()
			if err != nil {
				return fmt.Errorf("serialize job: %w", err)
			}
			_, err = elastic.GetDoc[string, *jobs.SerializedJob](
				ctx,
				Manager.store.GetIndexRO(service.ScheduleIndex),
				serialized.JobDetail().JobKey().String(),
			)
			if err != nil || errors.Is(err, elastic.ErrNotFound) {
				slogctx.Info(ctx, "Adding job.",
					slog.String("job_id", serialized.JobDetail().JobKey().String()),
				)
				if err = Manager.ScheduleJob(serialized.JobDetail(), serialized.Trigger()); err != nil {
					return fmt.Errorf("schedule get new feeds job: %w", err)
				}
			}
			return nil
		})
	}

	if err := startupTasks.Wait(); err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

	return nil
}

func LoadUpdateFeedJobs(ctx context.Context, feedSvc *service.FeedService) error {
	// Gather all current feed jobs.
	jobKeys, err := Manager.GetJobKeys(matcher.NewJobGroup(&matcher.StringEquals, "update_feed"))
	if err != nil {
		panic(fmt.Errorf("get existing jobs: %w", err))
	}

	// Extract [models.FeedID].
	feedIDs := make([]models.FeedID, 0, len(jobKeys))
	for key := range slices.Values(jobKeys) {
		feedIDs = append(feedIDs, strings.TrimPrefix(key.String(), "update_feed::"))
	}

	// Find all feeds that don't have an existing job.
	joblessFeeds, err := elastic.SearchAll[*models.Feed](
		ctx,
		Manager.store.GetIndexRO(service.FeedsIndex),
		query.Bool(
			query.MustNot(
				query.Terms("feed_id", feedIDs),
			),
		),
		5000,
	)
	if err != nil {
		panic(fmt.Errorf("search feeds: %w", err))
	}
	if len(joblessFeeds) > 0 {
		slogctx.Info(ctx, "Found feeds without jobs.",
			slog.Int("count", len(joblessFeeds)),
		)
	}

	for feed := range slices.Values(joblessFeeds) {
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
			newJob, err := jobs.NewUpdateFeedJob(ctx, feedSvc, feed.GetID())
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
