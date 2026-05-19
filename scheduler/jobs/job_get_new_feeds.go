// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

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

	"github.com/goforj/godump"
	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
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
		return fmt.Errorf("unable to unmarshal job data: %w", err)
	}

	start := time.Now()

	schedulerAPI, ok := ctx.Value(schedulerAPICtxKey).(SchedulerAPI)
	if !ok || schedulerAPI == nil {
		return fmt.Errorf("%w: unable to get scheduler api from context", ErrExecuteJobFailed)
	}

	slogctx.FromCtx(ctx).DebugContext(ctx, "Looking for new feeds.",
		slog.Time("since", data.Checkpoint),
	)

	// Find new feeds created since last checkpoint of job.
	var (
		allFeeds models.Feeds
	)
	allFeeds, err = elastic.SearchAll[*models.Feed](
		ctx,
		schema.FeedsIndexRO(),
		// query.Since("created_at", state.Checkpoint),
		// Consider a feed new if it has either:
		// - last_fetched value of the unix epoch
		// - missing last_fetched field
		query.Bool(
			query.Should(
				query.Before("last_fetched", models.UnixEpoch),
				query.Bool(
					query.MustNot(
						query.Exists("last_fetched"),
					),
				),
			),
		),
		5000,
	)
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrExecuteJobFailed, err)
	}
	if len(allFeeds) > 0 {
		slogctx.FromCtx(ctx).DebugContext(ctx, "Found new feeds.",
			slog.Int("count", len(allFeeds)),
			slog.Any("feed_ids", allFeeds.GetIDs()),
		)
	}

	var wg sync.WaitGroup
	// Create new feed jobs where necessary.
	for feed := range slices.Values(allFeeds) {
		// Add additional feed details to logs.
		feedCtx := slogctx.With(ctx, "feed_id", feed.GetID())
		feedCtx = slogctx.With(feedCtx, "feed_name", feed.GetTitle())
		godump.Dump(feed)
		wg.Go(func() {
			jobKey := quartz.NewJobKeyWithGroup(feed.GetID(), "update_feed")
			switch existingJob, err := schedulerAPI.GetScheduledJob(jobKey); {
			case err != nil && models.HTTPStatus(err) != http.StatusNotFound && !errors.Is(err, quartz.ErrJobNotFound):
				// If we cannot ascertain if there is an existing scheduled job, skip this feed.
				slogctx.FromCtx(feedCtx).Warn("Unable to check for existing scheduled job.",
					slog.String("feed_id", feed.GetID()),
					slog.Any("error", err),
				)
			case errors.Is(err, quartz.ErrJobNotFound):
				// Determine and set the feed update interval.
				if err := feed.SetUpdateInterval(feedCtx); err != nil {
					slogctx.FromCtx(feedCtx).Warn("Unable to set an update interval for feed.",
						slog.Any("error", err),
					)
					return
				}
				// If there is no existing scheduled newJob, create one.
				newJob, err := NewUpdateFeedJob(ctx, feed.GetID())
				if err != nil {
					slogctx.FromCtx(feedCtx).Warn("Unable to create new update feed job for feed.",
						slog.Any("error", err),
					)
					return
				}
				// Schedule the new job.
				if err = schedulerAPI.ScheduleJob(newJob.JobDetail(), newJob.Trigger()); err != nil {
					slogctx.FromCtx(feedCtx).Error("Failed to schedule new job for feed.",
						slog.String("job_id", newJob.JobDetail().JobKey().String()),
						slog.String("job_schedule", newJob.Trigger().Description()),
						slog.Any("error", err),
					)
					return
				}
				slogctx.FromCtx(feedCtx).Debug("Added new job for feed.",
					slog.String("job_id", newJob.JobDetail().JobKey().String()),
					slog.String("job_schedule", newJob.Trigger().Description()),
				)
				// Do an initial run of the job.
				if err = newJob.JobDetail().Job().Execute(ctx); err != nil {
					slogctx.FromCtx(feedCtx).Error("Failed initial run of update feed job.",
						slog.String("job_id", newJob.JobDetail().JobKey().String()),
						slog.String("job_schedule", newJob.Trigger().Description()),
						slog.Any("error", err),
					)
				}
			case existingJob != nil:
				// Existing job found, ignore.
				slogctx.FromCtx(feedCtx).Debug("Existing job found, ignoring.",
					slog.String("job_id", existingJob.JobDetail().JobKey().String()),
					slog.String("feed_id", feed.GetID()),
				)
			default:
				// Unhandled result.
				slogctx.FromCtx(feedCtx).Debug("Unhandled result.",
					slog.String("feed_id", feed.GetID()),
				)
			}
		})
	}

	wg.Wait()

	// Update the job data (checkpoint).
	if err := job.JobData.MergeGetNewFeedsJob(GetNewFeedsJob{Checkpoint: time.Now().UTC()}); err != nil {
		return fmt.Errorf("update job data: %w", err)
	}
	if err := elastic.UpdateDoc(
		ctx,
		schema.SchedulerIndexRW(),
		job.JobDetail().JobKey().String(),
		job,
		elastic.WithDocAsUpsert(true),
		elastic.WithRefresh(true),
	); err != nil {
		return fmt.Errorf("update job: %w", err)
	}

	slogctx.FromCtx(ctx).Debug("Finished get new feeds job.",
		slog.Duration("took", time.Since(start)))

	return nil
}
