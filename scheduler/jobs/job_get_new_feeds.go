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
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
)

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
func NewGetNewFeedsJob() (*ScheduledJob, error) {
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
	jobStateID := "get_new_feeds_state"

	schedulerAPI, ok := ctx.Value(schedulerAPICtxKey).(SchedulerAPI)
	if !ok || schedulerAPI == nil {
		return fmt.Errorf("%w: unable to get scheduler api from context", ErrExecuteJobFailed)
	}

	state := &GetNewFeedsJobState{}
	if lastState, err := schedulerAPI.GetJobState(ctx, jobStateID); err != nil {
		if models.HTTPStatus(err) != http.StatusNotFound {
			return fmt.Errorf("%w: %s: %w", ErrExecuteJobFailed, job.Description(), err)
		}
		state.Checkpoint = time.Time{}
	} else {
		err = json.Unmarshal(lastState.JobData, state)
		if err != nil {
			return fmt.Errorf("%w: %s: %w", ErrExecuteJobFailed, job.Description(), err)
		}
	}
	slogctx.FromCtx(ctx).DebugContext(ctx, "Looking for new feeds.",
		slog.Time("since", state.Checkpoint),
	)

	// Find new feeds. We detect new feeds by those where the last_fetched value equals Unix Epoch, indicating they
	// don't have a job scheduled for updating their items.
	var (
		feeds models.Feeds
		err   error
	)
	feeds, err = elastic.SearchAll[*models.Feed](
		ctx,
		models.FeedsIndexRO,
		query.Term("last_fetched", models.UnixEpoch),
		defaultPaginationSize,
	)
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrExecuteJobFailed, job.Description(), err)
	}
	if len(feeds) > 0 {
		slogctx.FromCtx(ctx).DebugContext(ctx, "Found new feeds.",
			slog.Int("count", len(feeds)),
			slog.Any("feed_ids", feeds.GetIDs()),
		)
	}
	// Update the checkpoint.
	state.Checkpoint = time.Now().UTC()
	err = schedulerAPI.UpdateJobState(ctx, jobStateID, map[string]any{
		"job_data": state,
	})
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrExecuteJobFailed, job.Description(), err)
	}

	var wg sync.WaitGroup
	// Create new feed jobs where necessary.
	for feed := range slices.Values(feeds) {
		wg.Go(func() {
			jobKey := job.generateJobKey(feed.GetID(), job.JobType)
			existingJob, err := schedulerAPI.GetScheduledJob(jobKey)
			if err != nil && models.HTTPStatus(err) != http.StatusNotFound && !errors.Is(err, quartz.ErrJobNotFound) {
				// If we cannot ascertain if there is an existing scheduled job, skip this feed.
				slogctx.FromCtx(ctx).Warn("Unable to check for existing scheduled job.",
					slog.String("feed_id", feed.GetID()),
					slog.Any("error", err),
				)
				return
			}
			if existingJob == nil || !errors.Is(err, quartz.ErrJobNotFound) {
				// If there is no existing scheduled newJob, create one.
				newJob, err := NewUpdateFeedJob(
					feed.GetID(),
					feed.SourceURLs,
					newPollTrigger(defaultPollInterval, defaultPollJitter),
				)
				if err != nil {
					slogctx.FromCtx(ctx).Warn("Unable to create new update feed job for feed.",
						slog.String("feed_id", feed.GetID()),
						slog.Any("error", err),
					)
					return
				}
				// Schedule the new job.
				if err = schedulerAPI.ScheduleJob(newJob.JobDetail(), newJob.Trigger()); err != nil {
					slogctx.FromCtx(ctx).Error("Failed to schedule new job for feed.",
						slog.String("feed_id", feed.GetID()),
						slog.String("job_id", newJob.JobDetail().JobKey().String()),
						slog.String("job_schedule", newJob.Trigger().Description()),
						slog.Any("error", err),
					)
					return
				}
				slogctx.FromCtx(ctx).DebugContext(ctx, "Added job for feed.",
					slog.String("feed_id", feed.GetID()),
					slog.String("job_id", newJob.JobDetail().JobKey().String()),
					slog.String("job_schedule", newJob.Trigger().Description()),
				)
				// Do an initial run of the job.
				if err = newJob.JobDetail().Job().Execute(ctx); err != nil {
					slogctx.FromCtx(ctx).Error("Failed initial run of update feed job.",
						slog.String("feed_id", feed.GetID()),
						slog.String("job_id", newJob.JobDetail().JobKey().String()),
						slog.String("job_schedule", newJob.Trigger().Description()),
						slog.Any("error", err),
					)
				}
				return
			}
			slogctx.FromCtx(ctx).Error("Existing update feed job.",
				slog.String("feed_id", feed.GetID()),
				slog.String("job_id", existingJob.JobDetail().JobKey().String()),
				slog.String("job_schedule", existingJob.Trigger().Description()),
				slog.Any("error", err),
			)
		})
	}

	wg.Wait()

	return nil
}
