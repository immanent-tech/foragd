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
	"strings"
	"time"

	"github.com/reugn/go-quartz/matcher"
	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
)

func NewRestartFeedUpdatesJob() (*SerializedJob, error) {
	job := &SerializedJob{
		CreatedAt:      time.Now().UTC(),
		JobDescription: new("Restart Feed Updates"),
		JobKey:         quartz.NewJobKey(string(JobTypeRestartFeedUpdates)).String(),
		JobType:        JobTypeRestartFeedUpdates,
		JobNextRun:     models.UnixEpoch,
		JobTriggerType: TriggerTypePoll,
	}

	if err := job.JobTrigger.FromPollTrigger(*NewPollTrigger(24*time.Hour, 5*time.Minute)); err != nil {
		return nil, fmt.Errorf("create trigger: %w", err)
	}

	return job, nil
}

func ExecuteRestartFeedUpdates(ctx context.Context, job *SerializedJob) error {
	start := time.Now()

	schedulerAPI, ok := ctx.Value(schedulerAPICtxKey).(SchedulerAPI)
	if !ok || schedulerAPI == nil {
		return errors.New("unable to get scheduler api from context")
	}

	// Gather all current feed jobs. Match against the job group "update_feed".
	jobKeys, err := schedulerAPI.GetJobKeys(matcher.NewJobGroup(&matcher.StringEquals, "update_feed"))
	if err != nil {
		return fmt.Errorf("get existing jobs: %w", err)
	}

	// Extract [models.FeedID].
	feedIDs := make([]models.FeedID, 0, len(jobKeys))
	for key := range slices.Values(jobKeys) {
		feedIDs = append(feedIDs, strings.TrimPrefix(key.String(), "update_feed::"))
	}

	// Find all feeds that don't have an existing job.
	joblessFeeds, err := elastic.SearchAll[*models.Feed](
		ctx,
		schema.FeedsIndexRO(),
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

	// Create and schedule jobs for feeds missing an update job.
	for feed := range slices.Values(joblessFeeds) {
		// Create a job for the feed.
		jobKey := quartz.NewJobKeyWithGroup(feed.GetID(), "update_feed")
		// Check if there is an existing scheduled job.
		switch existingJob, err := schedulerAPI.GetScheduledJob(jobKey); {
		case err != nil && models.HTTPStatus(err) != http.StatusNotFound && !errors.Is(err, quartz.ErrJobNotFound):
			// If we cannot ascertain if there is an existing scheduled job, skip this feed.
			slogctx.Warn(ctx, "Unable to check for existing scheduled job.",
				slog.String("feed_id", feed.GetID()),
				slog.Any("error", err),
			)
		case errors.Is(err, quartz.ErrJobNotFound):
			// If there is no existing scheduled newJob, create one.
			newJob, err := NewUpdateFeedJob(ctx, feed.GetID())
			if err != nil {
				slogctx.Warn(ctx, "Unable to create new update feed job for feed.",
					slog.Any("error", err),
				)
				continue
			}

			// Schedule the new job.
			if err = schedulerAPI.ScheduleJob(newJob.JobDetail(), newJob.Trigger()); err != nil {
				slogctx.Error(ctx, "Failed to schedule new job for feed.",
					slog.String("job_id", newJob.JobDetail().JobKey().String()),
					slog.String("job_schedule", newJob.Trigger().Description()),
					slog.Any("error", err),
				)
				continue
			}
			slogctx.Debug(ctx, "Added new job for feed.",
				slog.String("job_id", newJob.JobDetail().JobKey().String()),
				slog.String("job_schedule", newJob.Trigger().Description()),
			)
		case existingJob != nil:
			// Existing job found, ignore.
			slogctx.Debug(ctx, "Existing job found, ignoring.",
				slog.String("job_id", existingJob.JobDetail().JobKey().String()),
				slog.String("feed_id", feed.GetID()),
			)
		default:
			// Unhandled result.
			slogctx.Debug(ctx, "Unhandled result.",
				slog.String("feed_id", feed.GetID()),
			)
		}

	}

	slogctx.FromCtx(ctx).Debug("Finished restarting feed updates.",
		slog.Duration("took", time.Since(start)))

	return nil
}
