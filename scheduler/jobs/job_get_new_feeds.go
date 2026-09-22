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
	"net/http"
	"slices"
	"sync"
	"time"

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
	data, err := job.JobData.AsGetNewFeedsJob()
	if err != nil {
		return fmt.Errorf("unmarshal job data: %w", err)
	}

	schedulerAPI := SchedulerAPIFromCtx(ctx)
	if schedulerAPI == nil {
		return errors.New("cannot execute: no scheduler API in context")
	}

	feedSvc := FeedSvcFromCtx(ctx)
	if feedSvc == nil {
		return errors.New("cannot execute: no feed service in context")
	}

	start := time.Now()

	slogctx.Debug(ctx, "Looking for new feeds.",
		slog.Time("since", data.Checkpoint),
	)

	// Find new feeds.
	newFeeds, err := feedSvc.GetNewFeedsSince(ctx, models.UnixEpoch)
	if len(newFeeds) > 0 {
		slogctx.Debug(ctx, "Found new feeds.",
			slog.Int("count", len(newFeeds)),
			slog.Any("feed_ids", newFeeds.GetIDs()),
		)
	}

	var wg sync.WaitGroup
	// Create new feed jobs where necessary.
	for feed := range slices.Values(newFeeds) {
		// Add additional feed details to logs.
		feedCtx := slogctx.With(ctx, "feed_id", feed.GetID())
		feedCtx = slogctx.With(feedCtx, "feed_name", feed.GetTitle())
		wg.Go(func() {
			addFeedJob(feedCtx, feedSvc, feed)
		})
	}

	wg.Wait()

	// Update the job data (checkpoint).
	if err := job.JobData.MergeGetNewFeedsJob(GetNewFeedsJob{Checkpoint: time.Now().UTC()}); err != nil {
		return fmt.Errorf("update job data: %w", err)
	}
	if err := schedulerAPI.UpdateSerializedJob(ctx, job); err != nil {
		return fmt.Errorf("update serialized job: %w", err)
	}

	slogctx.Debug(ctx, "Finished get new feeds job.",
		slog.Duration("took", time.Since(start)))

	return nil
}

func addFeedJob(ctx context.Context, feedSvc *service.FeedService, feed *models.Feed) {
	schedulerAPI, ok := ctx.Value(schedulerAPICtxKey).(SchedulerAPI)
	if !ok || schedulerAPI == nil {
		slogctx.Error(ctx, "Unable to get scheduler API from context.")
	}

	jobKey := quartz.NewJobKeyWithGroup(feed.GetID(), "update_feed")
	switch existingJob, err := schedulerAPI.GetScheduledJob(jobKey); {
	case err != nil && models.HTTPStatus(err) != http.StatusNotFound && !errors.Is(err, quartz.ErrJobNotFound):
		// If we cannot ascertain if there is an existing scheduled job, skip this feed.
		slogctx.Warn(ctx, "Unable to check for existing scheduled job.",
			slog.String("feed_id", feed.GetID()),
			slog.Any("error", err),
		)
	case errors.Is(err, quartz.ErrJobNotFound):
		// If there is no existing scheduled newJob, create one.
		newJob, err := NewUpdateFeedJob(ctx, feedSvc, feed.GetID())
		if err != nil {
			slogctx.Warn(ctx, "Unable to create new update feed job for feed.",
				slog.Any("error", err),
			)
			return
		}
		// Schedule the new job.
		if err = schedulerAPI.ScheduleJob(newJob.JobDetail(), newJob.Trigger()); err != nil {
			slogctx.Error(ctx, "Failed to schedule new job for feed.",
				slog.String("job_id", newJob.JobDetail().JobKey().String()),
				slog.String("job_schedule", newJob.Trigger().Description()),
				slog.Any("error", err),
			)
			return
		}
		slogctx.Info(ctx, "Added new job for feed.",
			slog.String("job_id", newJob.JobDetail().JobKey().String()),
			slog.String("job_schedule", newJob.Trigger().Description()),
		)
		// Do an initial run of the job.
		if err = newJob.JobDetail().Job().Execute(ctx); err != nil {
			slogctx.Warn(ctx, "Failed initial run of update feed job. Pausing.",
				slog.String("job_id", newJob.JobDetail().JobKey().String()),
				slog.String("job_schedule", newJob.Trigger().Description()),
				slog.Any("error", err),
			)
			if err := schedulerAPI.PauseJob(newJob.getJobKey()); err != nil {
				slogctx.Error(ctx, "Unable to pause failing job.",
					slog.String("job_id", newJob.JobDetail().JobKey().String()),
					slog.String("job_schedule", newJob.Trigger().Description()),
					slog.Any("error", err),
				)
			}
			feed.LastFetched = time.Now().UTC()
			if err := feedSvc.UpdateFeed(ctx, feed); err != nil {
				slogctx.Error(ctx, "Unable to update last fetched.",
					slog.String("job_id", newJob.JobDetail().JobKey().String()),
					slog.String("job_schedule", newJob.Trigger().Description()),
					slog.Any("error", err),
				)
			}
		}
	case existingJob != nil:
		// Existing job found, ignore.
		slogctx.Debug(ctx, "Existing job found, ignoring.",
			slog.String("job_id", existingJob.JobDetail().JobKey().String()),
			slog.String("feed_id", feed.GetID()),
		)
		feed.LastFetched = time.Now().UTC()
		if err := feedSvc.UpdateFeed(ctx, feed); err != nil {
			slogctx.Error(ctx, "Unable to update last fetched.",
				slog.String("job_id", existingJob.JobDetail().JobKey().String()),
				slog.String("job_schedule", existingJob.Trigger().Description()),
				slog.Any("error", err),
			)
		}
	default:
		// Unhandled result.
		slogctx.Debug(ctx, "Unhandled result.",
			slog.String("feed_id", feed.GetID()),
		)
	}
}
