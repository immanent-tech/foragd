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
	"syscall"
	"time"

	quartzlogger "github.com/reugn/go-quartz/logger"
	"github.com/reugn/go-quartz/matcher"
	"github.com/reugn/go-quartz/quartz"

	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/go-base/logging"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	gerror "github.com/immanent-tech/foragd/providers/google/error"
	"github.com/immanent-tech/foragd/providers/resend"
	"github.com/immanent-tech/foragd/scheduler/jobs"
	"github.com/immanent-tech/foragd/scheduler/queue"
	"github.com/immanent-tech/foragd/service"
)

const (
	defaultOutdatedThreshold = time.Hour
	gracefulShutdownTimeout  = 30 * time.Second
)

// Manager contains data for managing a scheduler instance.
type Manager struct {
	quartz.Scheduler

	queue quartz.JobQueue
	store *service.ElasticService
}

// New will create a [Manager] containing the scheduler and job queue. New does not start the scheduler; call
// Run() to start it. The returned [Manager] can however be used to manage jobs (schedule/delete/query jobs).
func New() (*Manager, error) {
	logger := logging.New()
	// Create distributed queue instance.
	jobQueue, err := queue.NewJobQueue(logger)
	if err != nil {
		return nil, fmt.Errorf("new job queue: %w", err)
	}

	// Create scheduler instance.
	scheduler, err := quartz.NewStdScheduler(
		quartz.WithWorkerLimit(runtime.GOMAXPROCS(0)),
		quartz.WithOutdatedThreshold(defaultOutdatedThreshold),
		quartz.WithRetryInterval(time.Second),
		quartz.WithQueue(jobQueue, &sync.Mutex{}),
		quartz.WithLogger(quartzlogger.NewSlogLogger(context.Background(), logger)),
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

// Run starts the scheduler, including setting up background services, loading context for job queue and initialising
// admin jobs.
func (m *Manager) Run() error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer cancelFunc()
	ctx = slogctx.NewCtx(ctx, logging.New())

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
	m.Shutdown()

	return nil
}

// Shutdown performs shutdown logic for the scheduler, including safe shutdown of background services used by the
// scheduler and jobs.
func (m *Manager) Shutdown() {
	shutdownCtx, cancel := context.WithTimeoutCause(
		context.Background(),
		gracefulShutdownTimeout,
		errors.New("graceful shutdown timeout"),
	)
	shutdownCtx = slogctx.NewCtx(shutdownCtx, logging.New())
	defer cancel()

	shutdownTasks, tasksCtx := errgroup.WithContext(shutdownCtx)
	defer tasksCtx.Done()

	shutdownTasks.Go(func() error {
		return bulk.Shutdown(tasksCtx)
	})

	shutdownTasks.Go(func() error {
		return elastic.Shutdown(shutdownCtx)
	})

	shutdownTasks.Go(func() error {
		gerror.CloseClient()
		return nil
	})

	if err := shutdownTasks.Wait(); err != nil {
		slogctx.Warn(shutdownCtx, "Error occurred during scheduler shutdown.", slog.Any("error", err))
	}

	m.Stop()

	slogctx.FromCtx(shutdownCtx).Debug("Scheduler stopped.",
		slog.Time("stop_time", time.Now()),
	)
}

// InitAdminJobs loads the listed jobs into the scheduler. These are administrative jobs that should always be
// scheduled.
func (m *Manager) InitAdminJobs(ctx context.Context) error {
	// Load job services if needed.
	if jobServices := jobs.ServicesFromCtx(ctx); jobServices == nil {
		// Store various objects in the context for access by jobs:
		jobServices, err := m.generateJobServices(ctx)
		if err != nil {
			return fmt.Errorf("load job services: %w", err)
		}
		ctx = jobs.ServicesToCtx(ctx, jobServices)
	}

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

// LoadUpdateFeedJobs will ensure that all feeds have an update job scheduled.
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
		feedCtx := slogctx.With(ctx,
			"feed_id", feed.GetID(),
			"feed_name", feed.GetTitle(),
		)
		if slices.ContainsFunc(jobKeys, func(e *quartz.JobKey) bool {
			return e.Name() == feed.GetID()
		}) {
			slogctx.Warn(feedCtx, "Existing job found.")
			continue
		}
		wg.Go(func() {
			if err := jobs.AddFeedJob(feedCtx, m, feedSvc, feed); err != nil {
				slogctx.Error(feedCtx, "Could not add job for feed.",
					slog.Any("error", err),
				)
			}
		})
	}

	wg.Wait()

	return nil
}

func (m *Manager) generateJobServices(ctx context.Context) (*jobs.Services, error) {
	// Load all independent components concurrently. Many of these do network I/O (OIDC discovery, management tokens,
	// Elasticsearch, etc.), so doing them serially is the main startup cost.
	g, _ := errgroup.WithContext(ctx)

	bulkIndexerF := loadAsync(g, "bulk indexer", func() (*bulk.Indexer, error) {
		return bulk.NewIndexer(ctx, bulk.WithFlushInterval(time.Minute, 5*time.Second))
	})
	feedSvcF := loadAsync(g, "feed service", service.LoadFeedService)
	userSvcF := loadAsync(g, "user service", service.LoadUserService)
	importSvcF := loadAsync(g, "import service", service.NewImportService)
	emailSenderF := loadAsync(g, "email sender", resend.NewSender)

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("load services: %w", err)
	}

	return &jobs.Services{
		Scheduler:   m,
		Elastic:     m.store,
		Feeds:       feedSvcF(),
		Imports:     importSvcF(),
		Users:       userSvcF(),
		Indexer:     bulkIndexerF(),
		EmailSender: emailSenderF(),
	}, nil
}

// loadAsync runs fn in the errgroup and returns a getter for the result. The getter must only be called after g.Wait()
// has returned nil. The type is inferred, so no explicit service types are needed at the call site. It also logs how
// long each step took, which shows what dominates startup.
func loadAsync[T any](g *errgroup.Group, name string, fn func() (T, error)) func() T {
	var v T
	g.Go(func() error {
		start := time.Now()
		res, err := fn()
		if err != nil {
			return fmt.Errorf("load %s: %w", name, err)
		}
		v = res
		slog.Debug("Loaded component.", slog.String("component", name), slog.Duration("took", time.Since(start)))
		return nil
	})
	return func() T { return v }
}
