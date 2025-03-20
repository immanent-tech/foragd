// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/reugn/go-quartz/quartz"

	"github.com/joshuar/go-feed-me/internal/api"
	"github.com/joshuar/go-feed-me/internal/config"
	"github.com/joshuar/go-feed-me/internal/id"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/bulk"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/schema"
)

var (
	ErrRunFailed           = errors.New("failed to run scheduler")
	ErrFetchNewFeedsFailed = errors.New("could not fetch new feeds")
)

type DataAPI interface {
	GetNewFeedsSince(ctx context.Context, since time.Time) ([]models.APIFeed, error)
	GetFeedJobState(ctx context.Context, feedID models.FeedID) (*api.FeedState, error)
	UpdateFeedJobState(ctx context.Context, state *api.FeedState) error
	AddItems(ctx context.Context, items ...models.Item) (*bulk.Response, error)
}

type Manager struct {
	id         string
	db         *elastic.Client
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

	db := &elastic.ElasticAPI{
		API: esClient.GetAPI(),
	}

	ctx = FeedManagementAPIToCtx(ctx, db)
	ctx = elastic.FeedsIndexToCtx(ctx, schema.FeedsSchemaPrefix)
	ctx = elastic.ItemsIndexToCtx(ctx, schema.FeedItemsSchemaPrefix+"_"+config.Environment())

	jobQueue, err := NewJobQueue(ctx, esClient)
	if err != nil {
		return errors.Join(ErrRunFailed, err)
	}

	scheduler, err := quartz.NewStdScheduler(
		quartz.WithOutdatedThreshold(50*time.Second),
		quartz.WithQueue(jobQueue, &sync.Mutex{}),
	)
	if err != nil {
		return errors.Join(ErrRunFailed, err)
	}

	manager = &Manager{
		id:         schedulerID,
		db:         esClient,
		queue:      jobQueue,
		scheduler:  scheduler,
		logger:     logging.FromContext(ctx).WithGroup("scheduler"),
		checkpoint: time.Time{},
	}

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

	manager.logger.Info("Starting scheduler.")
	scheduler.Start(ctx)

	<-ctx.Done()

	manager.logger.Info("Stopping scheduler.")

	return nil
}

func (m *Manager) CheckFeeds(ctx context.Context) error {
	esapi := FeedManagementAPIFromCtx(ctx)
	if esapi == nil {
		return errors.Join(ErrExecuteJobFailed, fmt.Errorf("no feed management api in context"))
	}

	feeds, err := esapi.GetNewFeedsSince(ctx, m.checkpoint)
	if err != nil {
		return errors.Join(ErrFetchNewFeedsFailed, err)
	}

	m.checkpoint = time.Now().UTC()

	for _, feed := range feeds {
		var job quartz.ScheduledJob
		// Fetch any existing job.
		if existingJob, err := m.queue.Get(GenerateJobKey(feed.GetID())); err == nil {
			if details, ok := existingJob.(*ScheduledJob); ok {
				// If the existing job is scheduled by this scheduler instance,
				// ignore this feed.
				if details.SchedulerID == m.id {
					continue
				}
				// Otherwise, set the scheduler ID for the job to this scheduler
				details.SchedulerID = m.id
				// Delete the existing job.
				if err = m.scheduler.DeleteJob(GenerateJobKey(feed.GetID())); err != nil {
					m.logger.Warn("Could not reset existing job for feed .",
						slog.String("feed_id", feed.GetID()),
						slog.Any("error", err))

					continue
				}
				// Re-schedule the job.
				job = quartz.ScheduledJob(details)
			}
		} else {
			job, err = NewFeedJob(feed)
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
			slog.String("feed_id", feed.GetID()),
			slog.String("feed_title", feed.GetTitle()),
			slog.String("schedule", job.Trigger().Description()),
		)
	}

	return nil
}
