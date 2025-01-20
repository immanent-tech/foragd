// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/reugn/go-quartz/quartz"

	"github.com/joshuar/go-feed-me/internal/id"
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
}

type Manager struct {
	id         string
	db         databaseAPI
	queue      quartz.JobQueue
	scheduler  quartz.Scheduler
	logger     *slog.Logger
	checkpoint time.Time
}

var manager *Manager

func Run(ctx context.Context) error {
	schedulerID, err := id.NewID(id.Scheduler)
	if err != nil {
		return errors.Join(ErrRunFailed, err)
	}

	esClient, err := elastic.Connect(ctx)
	if err != nil {
		return errors.Join(ErrRunFailed, err)
	}

	ctx = models.FeedManagementAPIToCtx(ctx, esClient)

	jobQueue := elastic.NewJobQueue(ctx, esClient)
	scheduler := quartz.NewStdSchedulerWithOptions(quartz.StdSchedulerOptions{
		OutdatedThreshold: 50 * time.Second, // considering file system I/O latency
	}, jobQueue, nil)

	manager = &Manager{
		id:         schedulerID,
		db:         esClient,
		queue:      jobQueue,
		scheduler:  scheduler,
		logger:     logging.FromContext(ctx).WithGroup("scheduler"),
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

	scheduledJobs, err := jobQueue.ScheduledJobs(nil)
	if err != nil {
		manager.logger.Error("Failed to fetch scheduled jobs.",
			slog.Any("error", err))
		return errors.Join(ErrRunFailed, err)
	}

	jobNames := make([]string, 0, len(scheduledJobs))

	for _, job := range scheduledJobs {
		jobNames = append(jobNames, job.JobDetail().JobKey().String())
	}

	manager.logger.Debug("Jobs in queue: %s", slog.Any("jobs", jobNames))

	return nil
}

func (m *Manager) CheckFeeds(ctx context.Context) error {
	feeds, err := m.db.GetNewFeedsSince(ctx, m.checkpoint)
	if err != nil {
		return errors.Join(ErrFetchNewFeedsFailed, err)
	}

	m.checkpoint = time.Now().UTC()

	for _, feed := range feeds {
		var (
			job quartz.ScheduledJob
			err error
		)
		// Fetch any existing job.
		existingJob, err := m.queue.Get(models.GenerateJobKey(feed.GetID()))
		if err != nil {
			m.logger.Warn("Could not check for scheduled jobs.",
				slog.Any("error", err))
			continue
		}

		if existingJob != nil {
			if details, ok := existingJob.(*models.ScheduledJob); ok {
				// If the existing job is scheduled by this scheduler instance,
				// ignore this feed.
				if details.SchedulerID == m.id {
					continue
				}
				// Otherwise, set the scheduler ID for the job to this scheduler
				details.SchedulerID = m.id
				// Delete the existing job.
				if err = m.scheduler.DeleteJob(models.GenerateJobKey(feed.GetID())); err != nil {
					m.logger.Warn("Could not reset existing job for feed .",
						slog.String("feed_id", feed.GetID()),
						slog.Any("error", err))

					continue
				}
				// Re-schedule the job.
				job = quartz.ScheduledJob(details)
			}
		} else {
			job, err = models.NewFeedJob(feed)
			if err != nil {
				m.logger.Warn("Failed to schedule job for feed.",
					slog.String("feed_id", feed.GetID()),
					slog.Any("error", err))

				continue
			}
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
			slog.String("schedule", job.Trigger().Description()),
		)
	}

	return nil
}
