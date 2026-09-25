/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"slices"
	"sync"
	"time"

	"github.com/reugn/go-quartz/matcher"
	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/service"
)

// NewGetNewFeedsJob creates a job for checking for new feeds.
func NewGetNewFeedsJob() (*SerializedJob, error) {
	job := &SerializedJob{
		CreatedAt:      time.Now().UTC(),
		JobDescription: new("Find and schedule jobs to update feeds."),
		JobKey:         quartz.NewJobKey(string(JobTypeGetNewFeeds)).String(),
		JobType:        JobTypeGetNewFeeds,
		JobNextRun:     models.UnixEpoch,
		JobTriggerType: TriggerTypePoll,
	}

	if err := job.JobData.FromGetNewFeedsJob(GetNewFeedsJob{Checkpoint: models.UnixEpoch}); err != nil {
		return nil, fmt.Errorf("create job data: %w", err)
	}

	if err := job.JobTrigger.FromPollTrigger(*NewPollTrigger(DefaultPollInterval, DefaultPollJitter)); err != nil {
		return nil, fmt.Errorf("create trigger: %w", err)
	}

	return job, nil
}

// ExecuteGetNewFeeds runs a job that will look for newly added feeds and schedule new jobs to fetch item updates for
// them.
func ExecuteGetNewFeeds(ctx context.Context, job *SerializedJob) error {
	schedulerAPI := SchedulerAPIFromCtx(ctx)
	if schedulerAPI == nil {
		return errors.New("cannot execute: no scheduler API in context")
	}

	feedSvc := FeedSvcFromCtx(ctx)
	if feedSvc == nil {
		return errors.New("cannot execute: no feed service in context")
	}

	start := time.Now()

	slogctx.Debug(ctx, "Looking for new feeds.")

	// Find new feeds.
	newFeeds, err := feedSvc.GetNewFeedsSince(ctx, models.UnixEpoch)
	if err != nil {
		return fmt.Errorf("get new feeds since: %w", err)
	}
	if len(newFeeds) > 0 {
		slogctx.Debug(ctx, "Found new feeds.",
			slog.Int("count", len(newFeeds)),
			slog.Any("feed_ids", newFeeds.GetIDs()),
		)
	}

	// Get job keys for existing feed jobs.
	existingJobKeys, err := schedulerAPI.GetJobKeys(matcher.JobGroupEquals("update_feed"))
	if err != nil {
		return fmt.Errorf("get job keys: %w", err)
	}

	maxConcurrentImports := runtime.GOMAXPROCS(0)
	feedCh := make(chan *models.Feed, maxConcurrentImports*5)
	var wg sync.WaitGroup

	for range maxConcurrentImports {
		// Create new feed jobs where necessary.
		for feed := range slices.Values(newFeeds) {
			// Add additional feed details to logs.
			feedCtx := slogctx.With(ctx, "feed_id", feed.GetID())
			feedCtx = slogctx.With(feedCtx, "feed_name", feed.GetTitle())
			// Skip adding if there is already an existing job.
			if slices.ContainsFunc(existingJobKeys, func(e *quartz.JobKey) bool {
				return e.Name() == feed.GetID()
			}) {
				continue
			}
			// Add and process update feed job.
			wg.Go(func() {
				if err := AddFeedJob(feedCtx, schedulerAPI, feedSvc, feed); err != nil {
					slogctx.Error(ctx, "Unable to add feed job",
						slog.Any("error", err),
					)
				}
			})
		}
	}

	for feed := range slices.Values(newFeeds) {
		select {
		case feedCh <- feed:
		case <-ctx.Done():
			close(feedCh)
			wg.Wait()
			return ctx.Err()
		}

	}
	close(feedCh)

	wg.Wait()

	slogctx.Debug(ctx, "Finished get new feeds job.",
		slog.Duration("took", time.Since(start)))

	return nil
}

func AddFeedJob(
	ctx context.Context,
	scheduler SchedulerAPI,
	feedSvc *service.FeedService,
	feed *models.Feed,
) error {
	// Create update feed job.
	job, err := NewUpdateFeedJob(ctx, feedSvc, feed.GetID())
	if err != nil {
		return fmt.Errorf("new update feed job: %w", err)
	}

	// Schedule the new job.
	if err = scheduler.ScheduleJob(job.JobDetail(), job.Trigger()); err != nil {
		return fmt.Errorf("schedule update feed job: %w", err)
	}
	slogctx.Info(ctx, "Added new job for feed.",
		slog.String("job_id", job.JobDetail().JobKey().String()),
		slog.String("job_schedule", job.Trigger().Description()),
	)

	// Do an initial run of the job.
	if err = job.JobDetail().Job().Execute(ctx); err != nil {
		slogctx.Warn(ctx, "Failed initial run of update feed job. Pausing.",
			slog.String("job_id", job.JobDetail().JobKey().String()),
			slog.String("job_schedule", job.Trigger().Description()),
			slog.Any("error", err),
		)
		feed.LastFetched = time.Now().UTC()
		if err := feedSvc.UpdateFeed(ctx, feed); err != nil {
			slogctx.Error(ctx, "Unable to update last fetched.",
				slog.String("job_id", job.JobDetail().JobKey().String()),
				slog.String("job_schedule", job.Trigger().Description()),
				slog.Any("error", err),
			)
		}
	}
	return nil
}
