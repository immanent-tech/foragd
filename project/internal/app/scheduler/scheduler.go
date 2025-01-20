// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/reugn/go-quartz/quartz"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic"
)

var (
	ErrRunFailed           = errors.New("failed to run scheduler")
	ErrFetchNewFeedsFailed = errors.New("could not fetch new feeds")
)

type databaseAPI interface {
	GetNewFeedsSince(ctx context.Context, since time.Time) ([]models.APIFeed, error)
	AddItems(ctx context.Context, items ...models.Item) error
	FeedJobExists(ctx context.Context, feedID models.FeedID) (bool, error)
}

type Manager struct {
	db         databaseAPI
	queue      quartz.JobQueue
	scheduler  quartz.Scheduler
	logger     *slog.Logger
	checkpoint time.Time
}

var manager *Manager

func Run(ctx context.Context, env string) error {
	esClient, err := elastic.Connect(ctx)
	if err != nil {
		return errors.Join(ErrRunFailed, err)
	}

	ctx = models.FeedManagementAPIToCtx(ctx, esClient)

	logger := logging.FromContext(ctx).WithGroup("scheduler")

	jobQueue := elastic.NewJobQueue(ctx, esClient)
	scheduler := quartz.NewStdSchedulerWithOptions(quartz.StdSchedulerOptions{
		OutdatedThreshold: 50 * time.Second, // considering file system I/O latency
	}, jobQueue, nil)

	manager = &Manager{
		db:         esClient,
		queue:      jobQueue,
		scheduler:  scheduler,
		logger:     logger,
		checkpoint: time.Time{},
	}

	manager.logger.Debug("Scheduler started.")

	ticker := time.NewTicker(time.Minute)

	jobChecker := func() {
		if err := manager.CheckFeeds(ctx); err != nil {
			manager.logger.Error("Checking for new feeds failed.",
				slog.Any("error", err))
		}
	}

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

	// jobQueueSize, err := jobQueue.Size()
	// if err != nil {
	// 	logger.Errorf("Failed to fetch job queue size: %s", err)
	// 	return errors.Join(ErrRunFailed, err)
	// }

	scheduler.Start(ctx)

	<-ctx.Done()

	// scheduledJobs, err := jobQueue.ScheduledJobs(nil)
	// if err != nil {
	// 	manager.logger.Error("Failed to fetch scheduled jobs.",
	// 		slog.Any("error", err))
	// 	return errors.Join(ErrRunFailed, err)
	// }

	// jobNames := make([]string, 0, len(scheduledJobs))

	// for _, job := range scheduledJobs {
	// 	jobNames = append(jobNames, job.JobDetail().JobKey().String())
	// }

	return nil
}

func (m *Manager) CheckFeeds(ctx context.Context) error {
	feeds, err := m.db.GetNewFeedsSince(ctx, m.checkpoint)
	if err != nil {
		return errors.Join(ErrFetchNewFeedsFailed, err)
	}

	m.checkpoint = time.Now().UTC()

	for _, feed := range feeds {
		if found, err := m.db.FeedJobExists(ctx, models.FeedJobGroup+quartz.Sep+feed.GetID()); err != nil || found {
			m.logger.Warn("Not scheduling job for feed.",
				slog.String("feed_id", feed.GetID()),
				slog.Bool("found", found),
				slog.Any("error", err))

			continue
		}

		job, err := models.NewFeedJob(feed)
		if err != nil {
			m.logger.Warn("Failed to schedule job for feed.",
				slog.String("feed_id", feed.GetID()),
				slog.Any("error", err))

			continue
		}

		if err := m.scheduler.ScheduleJob(job.JobDetail(), job.Trigger()); err != nil {
			m.logger.Warn("Failed to schedule job for feed.",
				slog.String("feed_id", feed.GetID()),
				slog.Any("error", err))

			continue
		}

		m.logger.Debug("Adding job for feed.",
			slog.String("feed_id", feed.ID),
			slog.String("feed_title", feed.Title),
			slog.String("schedule", job.Schedule),
		)
	}

	return nil
}
