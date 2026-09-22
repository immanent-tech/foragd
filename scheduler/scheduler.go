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

// Manager contains data for managing a scheduler instance.
type Manager struct {
	quartz.Scheduler

	queue quartz.JobQueue
	store *service.ElasticService
}

// NewManager will create a new manager object containing the scheduler and job queue.
func NewManager(ctx context.Context) (*Manager, error) {
	// Create distributed queue instance.
	jobQueue, err := queue.NewJobQueue(ctx)
	if err != nil {
		return nil, fmt.Errorf("new job queue: %w", err)
	}

	// Create scheduler instance.
	scheduler, err := quartz.NewStdScheduler(
		quartz.WithOutdatedThreshold(defaultOutdatedThreshold),
		quartz.WithRetryInterval(500*time.Millisecond),
		quartz.WithQueue(jobQueue, &sync.Mutex{}),
		quartz.WithLogger(logger.NewSlogLogger(ctx, slogctx.FromCtx(ctx))),
	)
	if err != nil {
		return nil, fmt.Errorf("new scheduler: %w", err)
	}

	// Load the elastic service.
	elasticSvc, err := service.LoadElasticService()
	if err != nil {
		return nil, fmt.Errorf("load elastic service: %w", err)
	}

	return &Manager{
		Scheduler: scheduler,
		queue:     jobQueue,
		store:     elasticSvc,
	}, nil
}

// Run starts the scheduler manager.
func (m *Manager) Run(ctx context.Context) error {
	appCfg, err := config.LoadAppConfig()
	if err != nil {
		return fmt.Errorf("load app config: %w", err)
	}

	// Store various objects in the context for access by jobs:
	// Elastic service.
	ctx = jobs.ElasticToCtx(ctx, m.store)
	// Bulk indexer.
	indexer, err := bulk.NewIndexer(ctx, bulk.WithFlushInterval(time.Minute, 5*time.Second))
	if err != nil {
		return fmt.Errorf("create indexer: %w", err)
	}
	ctx = jobs.IndexerToCtx(ctx, indexer)
	// Scheduler.
	ctx = jobs.SchedulerAPIToCtx(ctx, m)
	// HTTP client.
	httpClient, err := client.Load()
	if err != nil {
		return fmt.Errorf("load http client: %w", err)
	}
	httpClient = httpClient.SetHeader(
		"User-Agent",
		appCfg.GetAppName()+"/"+appCfg.GetAppVersion()+" (+https://foragd.app/policies/bot)",
	)
	ctx = jobs.HTTPClientToCtx(ctx, httpClient)
	// Item cache.
	itemsCache, err := cache.NewItemsCache()
	if err != nil {
		return fmt.Errorf("load items cache: %w", err)
	}
	ctx = jobs.ItemCacheToCtx(ctx, itemsCache)
	// Item service.
	itemSvc, err := service.LoadItemService()
	if err != nil {
		return fmt.Errorf("load item service: %w", err)
	}
	ctx = jobs.ItemSvcToCtx(ctx, itemSvc)
	// User service.
	userSvc, err := service.LoadUserService()
	if err != nil {
		return fmt.Errorf("load user service: %w", err)
	}
	ctx = jobs.UserSvcToCtx(ctx, userSvc)
	// Feed service.
	feedSvc, err := service.LoadFeedService()
	if err != nil {
		return fmt.Errorf("load feed service: %w", err)
	}
	ctx = jobs.FeedSvcToCtx(ctx, feedSvc)

	// Load all admin jobs as needed.
	if err := m.InitAdminJobs(ctx); err != nil {
		return fmt.Errorf("run scheduler startup tasks: %w", err)
	}

	// Start scheduling jobs.
	m.Start(ctx)
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
	m.Stop()

	slogctx.FromCtx(shutdownCtx).Debug("Scheduler stopped.",
		slog.Time("stop_time", time.Now()),
	)

	return nil
}

// // Clear will remove all jobs from the queue.
// func (m *Manager) Clear(ctx context.Context) error {
// 	if err := m.queue.Clear(); err != nil {
// 		return fmt.Errorf("clear job queue: %w", err)
// 	}
// 	return nil
// }

// InitAdminJobs loads the listed jobs into the scheduler. These are administrative jobs that should always be
// scheduled.
func (m *Manager) InitAdminJobs(ctx context.Context) error {
	// List of jobs to activate at startup.
	var startupJobs = []func() (*jobs.SerializedJob, error){
		jobs.NewGetNewFeedsJob,
		jobs.NewClearDeletedFeedsJob,
		jobs.NewDeleteExpiredSessionsJob,
		jobs.NewRestartFeedUpdatesJob,
	}

	startupTasks, tasksCtx := errgroup.WithContext(ctx)
	defer tasksCtx.Done()

	for newJobFunc := range slices.Values(startupJobs) {
		startupTasks.Go(func() error {
			serialized, err := newJobFunc()
			if err != nil {
				return fmt.Errorf("serialize job: %w", err)
			}
			keys, err := m.GetJobKeys(matcher.JobNameEquals(serialized.JobDetail().JobKey().Name()))
			if err != nil {
				return fmt.Errorf("get existing job details: %w", err)
			}
			if len(keys) > 0 {
				slogctx.Debug(ctx, "Job already scheduled.", slog.String("job_key", keys[0].String()))
				return nil
			}
			slogctx.Info(ctx, "Adding job.",
				slog.String("job_id", serialized.JobDetail().JobKey().String()),
			)
			if err = m.ScheduleJob(serialized.JobDetail(), serialized.Trigger()); err != nil {
				return fmt.Errorf("schedule get new feeds job: %w", err)
			}
			return nil
		})
	}

	if err := startupTasks.Wait(); err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

	return nil
}

func (m *Manager) LoadUpdateFeedJobs(ctx context.Context, feedSvc *service.FeedService) error {
	// Gather all current feed jobs.
	jobKeys, err := m.GetJobKeys(matcher.NewJobGroup(&matcher.StringEquals, "update_feed"))
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
		m.store.GetIndexRO(service.FeedsIndex),
		query.Bool(
			query.MustNot(
				query.Terms("feed_id", feedIDs),
			),
		),
		5000,
	)
	if err != nil {
		return fmt.Errorf("search feeds: %w", err)
	}
	if len(joblessFeeds) > 0 {
		slogctx.Info(ctx, "Found feeds without jobs.",
			slog.Int("count", len(joblessFeeds)),
		)
	}

	var wg sync.WaitGroup

	for feed := range slices.Values(joblessFeeds) {
		// Add additional feed details to logs.
		feedCtx := slogctx.With(ctx, "feed_id", feed.GetID())
		feedCtx = slogctx.With(feedCtx, "feed_name", feed.GetTitle())

		wg.Go(func() {
			if err := jobs.AddFeedJob(ctx, m, feedSvc, feed); err != nil {
				slogctx.Error(ctx, "Could not add job for feed.",
					slog.Any("error", err),
				)
			}
		})
	}

	wg.Wait()

	return nil
}

func (m *Manager) UpdateSerializedJob(ctx context.Context, job *jobs.SerializedJob) error {
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
