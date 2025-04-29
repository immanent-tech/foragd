// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/components/config"
	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/providers/elastic/bulk"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/providers/elastic/schema"
)

var (
	ErrRunFailed           = errors.New("failed to run scheduler")
	ErrFetchNewFeedsFailed = errors.New("could not fetch new feeds")
)

type DataAPI interface {
	GetFeed(ctx context.Context, id models.FeedID) (*models.Feed, error)
	FeedsSearchAll(ctx context.Context, queries ...query.Option) (models.Feeds, error)
	// GetNewFeedsSince(ctx context.Context, since time.Time) (models.Feeds, error)
	AddItems(ctx context.Context, items ...*models.Item) (*bulk.Response, error)
	MarkFeedUpdated(ctx context.Context, feedID models.FeedID) error
}

type Manager struct {
	id         string
	db         *elastic.API
	queue      quartz.JobQueue
	scheduler  quartz.Scheduler
	checkpoint time.Time
}

var manager *Manager

func Run(ctx context.Context) error {
	esClient, err := elastic.Connect(ctx)
	if err != nil {
		return errors.Join(ErrRunFailed, err)
	}

	db := &elastic.API{
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
		id:         models.NewID(models.SchedulerPFX),
		db:         esClient,
		queue:      jobQueue,
		scheduler:  scheduler,
		checkpoint: time.Time{},
	}

	ticker := time.NewTicker(time.Minute)

	jobChecker := func() {
		if err := manager.CheckFeeds(ctx); err != nil {
			slogctx.FromCtx(ctx).
				WithGroup("scheduler").
				Error("Checking for new feeds failed.",
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

	scheduler.Start(ctx)
	<-ctx.Done()
	scheduler.Stop()
	return nil
}

func (m *Manager) CheckFeeds(ctx context.Context) error {
	feeds, err := m.db.FeedsSearchAll(ctx, query.Since("created_at", m.checkpoint))
	// feeds, err := m.db.GetNewFeedsSince(ctx, m.checkpoint)
	if err != nil {
		return errors.Join(ErrFetchNewFeedsFailed, err)
	}

	m.checkpoint = time.Now().UTC()

	for feed := range slices.Values(feeds) {
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
					slogctx.FromCtx(ctx).Warn("Could not reset existing job for feed .",
						slog.String("feed_id", feed.GetID()),
						slog.Any("error", err))

					continue
				}
				// Re-schedule the job.
				job = quartz.ScheduledJob(details)
			}
		} else {
			job, err = NewFeedJob(feed.GetID(), feed.GetSourceURL(), NewPollTrigger(defaultPollInterval, defaultPollJitter))
			if err != nil {
				slogctx.FromCtx(ctx).Warn("Failed to schedule job for feed.",
					slog.String("feed_id", feed.GetID()),
					slog.Any("error", err))

				continue
			}
		}

		if err := m.scheduler.ScheduleJob(job.JobDetail(), job.Trigger()); err != nil {
			slogctx.FromCtx(ctx).Warn("Failed to schedule job for feed.",
				slog.String("feed_id", feed.GetID()),
				slog.Any("error", err))

			continue
		}

		slogctx.FromCtx(ctx).Debug("Adding job for feed.",
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

	return nil
}
