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
	"runtime"
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
		quartz.WithWorkerLimit(runtime.GOMAXPROCS(0)*2),
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
	// Store various objects in the context for access by jobs:
	jobServices, err := m.generateJobServices(ctx)
	if err != nil {
		return fmt.Errorf("load job services: %w", err)
	}
	ctx = jobs.ServicesToCtx(ctx, jobServices)

	// Load all admin jobs as needed.
	if err := m.InitAdminJobs(ctx); err != nil {
		return fmt.Errorf("run scheduler startup tasks: %w", err)
	}

	// Start scheduling jobs.
	m.Start(ctx)
	slogctx.Info(ctx, "Scheduler started.",
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

// InitAdminJobs loads the listed jobs into the scheduler. These are administrative jobs that should always be
// scheduled.
func (m *Manager) InitAdminJobs(ctx context.Context) error {
	// List of jobs to activate at startup.
	var startupJobs = []func() (*jobs.SerializedJob, error){
		jobs.NewGetNewFeedsJob,
		jobs.NewClearDeletedFeedsJob,
		jobs.NewDeleteExpiredSessionsJob,
		jobs.NewRestartFeedUpdatesJob,
		jobs.NewRunImportsJob,
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

	existingJobKeys, err := m.GetJobKeys(matcher.JobGroupEquals("update_feed"))
	if err != nil {
		return fmt.Errorf("get job keys: %w", err)
	}

	var wg sync.WaitGroup

	for feed := range slices.Values(joblessFeeds) {
		// Add additional feed details to logs.
		feedCtx := slogctx.With(ctx, "feed_id", feed.GetID())
		feedCtx = slogctx.With(feedCtx, "feed_name", feed.GetTitle())
		if slices.ContainsFunc(existingJobKeys, func(e *quartz.JobKey) bool {
			return e.Name() == feed.GetID()
		}) {
			slogctx.Warn(ctx, "Existing job found.")
			continue
		}
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

func (m *Manager) generateJobServices(ctx context.Context) (*jobs.Services, error) {
	appCfg, err := config.LoadAppConfig()
	if err != nil {
		return nil, fmt.Errorf("load app config: %w", err)
	}

	// Store various objects in the context for access by jobs:
	// Bulk indexer.
	indexer, err := bulk.NewIndexer(ctx, bulk.WithFlushInterval(time.Minute, 5*time.Second))
	if err != nil {
		return nil, fmt.Errorf("create indexer: %w", err)
	}
	// HTTP client.
	httpClient, err := client.Load()
	if err != nil {
		return nil, fmt.Errorf("load http client: %w", err)
	}
	httpClient = httpClient.SetHeader(
		"User-Agent",
		appCfg.GetAppName()+"/"+appCfg.GetAppVersion()+" (+https://foragd.app/policies/bot)",
	)
	// Item cache.
	itemsCache, err := cache.NewItemsCache()
	if err != nil {
		return nil, fmt.Errorf("load items cache: %w", err)
	}
	// User service.
	userSvc, err := service.LoadUserService()
	if err != nil {
		return nil, fmt.Errorf("load user service: %w", err)
	}
	// Feed service.
	feedSvc, err := service.LoadFeedService()
	if err != nil {
		return nil, fmt.Errorf("load feed service: %w", err)
	}
	// Import service
	importSvc, err := service.NewImportService()
	if err != nil {
		return nil, fmt.Errorf("load import service: %w", err)
	}

	return &jobs.Services{
		Scheduler:  m,
		Elastic:    m.store,
		Feeds:      feedSvc,
		Imports:    importSvc,
		Users:      userSvc,
		ItemsCache: itemsCache,
		Indexer:    indexer,
		HttpClient: httpClient,
	}, nil
}
