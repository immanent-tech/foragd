// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package scheduler contains code for the scheduler backend that handles managing background jobs for the application.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/config"
	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/providers/elastic/schema"
)

const (
	defaultOutdatedThreshold = 50 * time.Second
)

var ErrScheduler = errors.New("scheduler encountered an error")

// Manager contains data for managing a scheduler instance.
type Manager struct {
	id         string
	db         *elastic.API
	queue      quartz.JobQueue
	scheduler  quartz.Scheduler
	checkpoint time.Time
}

var manager *Manager

// Run starts the scheduler manager.
func Run(ctx context.Context) error {
	esClient, err := elastic.Connect(ctx)
	if err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

	db := &elastic.API{
		API: esClient.GetAPI(),
	}

	ctx = FeedManagementAPIToCtx(ctx, db)
	ctx = elastic.FeedsIndexToCtx(ctx, schema.FeedsSchemaPrefix)
	ctx = elastic.ItemsIndexToCtx(ctx, schema.ItemsSchemaPrefix+"_"+config.Environment())

	jobQueue, err := NewJobQueue(ctx, esClient)
	if err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

	logger := &logger{Logger: slogctx.FromCtx(ctx)}
	misfiredCh := make(chan quartz.ScheduledJob)
	scheduler, err := quartz.NewStdScheduler(
		quartz.WithOutdatedThreshold(defaultOutdatedThreshold),
		quartz.WithMisfiredChan(misfiredCh),
		quartz.WithQueue(jobQueue, &sync.Mutex{}),
		quartz.WithLogger(logger),
	)
	if err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

	manager = &Manager{
		id:         models.NewID(models.SchedulerPFX),
		db:         esClient,
		queue:      jobQueue,
		scheduler:  scheduler,
		checkpoint: time.Time{},
	}

	ticker := time.NewTicker(time.Minute)

	jobChecker := func() {
		err := manager.checkFeeds(ctx)
		if err != nil {
			slogctx.FromCtx(ctx).WithGroup("scheduler").Error("Checking for new feeds failed.",
				slog.Any("error", err),
			)
		}
	}

	// Run goroutine to check for new feeds.
	jobChecker()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				jobChecker()
			}
		}
	}()

	// Run goroutine to log misfired jobs.
	go func() {
		for misfiredJob := range misfiredCh {
			logger.Error("Job misfired.",
				slog.String("job_id", misfiredJob.JobDetail().JobKey().String()),
				slog.String("job_description", misfiredJob.JobDetail().Job().Description()),
			)
		}
	}()

	scheduler.Start(ctx)
	slogctx.FromCtx(ctx).Debug("Scheduler started.")
	<-ctx.Done()
	scheduler.Stop()
	slogctx.FromCtx(ctx).Debug("Scheduler stopped.")
	return nil
}

func (m *Manager) checkFeeds(ctx context.Context) error {
	// Get all new feeds created since last checkpoint.
	index := elastic.FeedsIndexFromCtx(ctx)
	if index == "" {
		return fmt.Errorf("%w: no feed index found", ErrScheduler)
	}
	var feeds models.Feeds
	feeds, err := elastic.SearchAll[*models.Feed](ctx, m.db.GetAPI(), index, query.Since("created_at", m.checkpoint), 1000)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrScheduler, err)
	}
	slogctx.FromCtx(ctx).Debug("Retrieved new feeds.",
		slog.Int("count", len(feeds)),
	)
	// Update the checkpoint.
	m.checkpoint = time.Now().UTC()
	// Get all existing feed jobs.
	existingJobs, err := m.scheduler.GetJobKeys()
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Could not get existing job keys from scheduler.",
			slog.Any("error", err),
		)
	}
	// Create new feed jobs where necessary.
	for feed := range slices.Values(feeds) {
		var (
			job quartz.ScheduledJob
			err error
		)
		job, err = NewFeedJob(feed.GetID(), feed.GetSourceURL(), NewPollTrigger(defaultPollInterval, defaultPollJitter))
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Failed to schedule job for feed.",
				slog.String("feed_id", feed.GetID()),
				slog.Any("error", err))

			continue
		}
		if slices.Contains(existingJobs, job.JobDetail().JobKey()) {
			err := m.scheduler.ResumeJob(job.JobDetail().JobKey())
			if err != nil {
				slogctx.FromCtx(ctx).Warn("Failed to resume job for feed.",
					slog.String("feed_id", feed.GetID()),
					slog.Any("error", err))
			}
		} else {
			err := m.scheduler.ScheduleJob(job.JobDetail(), job.Trigger())
			if err != nil {
				slogctx.FromCtx(ctx).Warn("Failed to schedule job for feed.",
					slog.String("feed_id", feed.GetID()),
					slog.Any("error", err))

				continue
			}
			slogctx.FromCtx(ctx).Debug("Added job for feed.",
				slog.Group("feed",
					slog.String("id", feed.GetID()),
					slog.String("title", feed.GetTitle()),
				),
				slog.Group("job",
					slog.String("id", job.JobDetail().JobKey().String()),
					slog.String("schedule", job.Trigger().Description()),
				),
			)
		}
	}

	return nil
}
