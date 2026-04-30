// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package jobs

import (
	"context"
	"encoding/json"
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
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
)

const jobTypeGetNewFeeds jobType = "get_new_feeds"

// GetNewFeedsJobData contains the data required by the GetNewFeeds job.
type GetNewFeedsJobData struct {
	// Interval is the interval on which to check for new feeds.
	Interval string `json:"interval"`
}

// GetNewFeedsJobState represents the state required by this job type.
type GetNewFeedsJobState struct {
	// Checkpoint is the timestamp when the job last checked for new feeds.
	Checkpoint time.Time `json:"checkpoint"`
}

// NewGetNewFeedsJob creates a job for checking for new feeds.
func NewGetNewFeedsJob(ctx context.Context) (*ScheduledJob, error) {
	job := &ScheduledJob{
		CreatedAt:      time.Now().UTC(),
		JobTriggerType: jobTriggerTypePoll,
		JobType:        jobTypeGetNewFeeds,
		JobDescription: "Find new feeds",
	}

	var (
		data []byte
		err  error
	)

	if _, err := fetchGetNewFeedsJobState(ctx); err != nil {
		return nil, fmt.Errorf("%w: set up job state: %w", ErrCreateJobFailed, err)
	}

	// Create trigger.
	data, err = json.Marshal(newPollTrigger(defaultPollInterval, defaultPollJitter))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateJobFailed, err)
	}
	job.JobTrigger = data

	// Create job data.
	data, err = json.Marshal(GetNewFeedsJobData{Interval: time.Minute.String()})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateJobFailed, err)
	}
	job.JobData = data

	return job, nil
}

// executeGetNewFeedsJob runs a job that will look for newly added feeds and schedule new jobs to fetch item updates for
// them.
func executeGetNewFeedsJob(ctx context.Context, job *ScheduledJob) error {
	state, err := fetchGetNewFeedsJobState(ctx)
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrExecuteJobFailed, job.Description(), err)
	}

	schedulerAPI, ok := ctx.Value(schedulerAPICtxKey).(SchedulerAPI)
	if !ok || schedulerAPI == nil {
		return fmt.Errorf("%w: unable to get scheduler api from context", ErrExecuteJobFailed)
	}

	slogctx.FromCtx(ctx).DebugContext(ctx, "Looking for new feeds.",
		slog.Time("since", state.Checkpoint),
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
		defaultPaginationSize,
	)
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrExecuteJobFailed, job.Description(), err)
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

		wg.Go(func() {
			jobKey := job.GenerateJobKey(feed.GetID(), string(job.JobType))
			existingJob, err := schedulerAPI.GetScheduledJob(jobKey)
			switch {
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
				newJob, err := NewUpdateFeedJob(
					feed.GetID(),
					newPollTrigger(feed.UpdateInterval, defaultPollJitter),
				)
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

	// Update the checkpoint.
	jobStateID := string(jobTypeGetNewFeeds) + "_state"
	state.Checkpoint = time.Now().UTC()
	if err = schedulerAPI.UpdateJobState(ctx, jobStateID, map[string]any{
		"job_data": state,
	}); err != nil {
		return fmt.Errorf("%w: %s: %w", ErrExecuteJobFailed, job.Description(), err)
	}

	return nil
}

func fetchGetNewFeedsJobState(ctx context.Context) (*GetNewFeedsJobState, error) {
	schedulerAPI, ok := ctx.Value(schedulerAPICtxKey).(SchedulerAPI)
	if !ok || schedulerAPI == nil {
		return nil, errors.New("unable to get scheduler api from context")
	}

	jobStateID := string(jobTypeGetNewFeeds) + "_state"

	state := &GetNewFeedsJobState{}
	if lastState, err := schedulerAPI.GetJobState(ctx, jobStateID); err != nil {
		if !errors.Is(err, elastic.ErrNotFound) {
			return nil, fmt.Errorf("get existing job state: %w", err)
		}
		slogctx.FromCtx(ctx).Debug("No existing job state. Creating new.")
		state.Checkpoint = time.Time{}
		if err = schedulerAPI.UpdateJobState(ctx, jobStateID, map[string]any{
			"job_data": state,
		}); err != nil {
			return nil, fmt.Errorf("create job state: %w", err)
		}
	} else {
		err = json.Unmarshal(lastState.JobData, state)
		if err != nil {
			return nil, fmt.Errorf("marshal job state: %w", err)
		}
	}
	return state, nil
}
