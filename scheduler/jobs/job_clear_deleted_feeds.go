// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
)

// ClearDeletedFeedsState represents the state required by this job type.
type ClearDeletedFeedsState struct {
	// Checkpoint is the timestamp when the job last checked for new feeds.
	Checkpoint time.Time `json:"checkpoint"`
}

// NewClearDeletedFeedsJob creates a job for checking for new feeds.
func NewClearDeletedFeedsJob() (*ScheduledJob, error) {
	job := &ScheduledJob{
		CreatedAt:      time.Now().UTC(),
		JobTriggerType: jobTriggerTypePoll,
		JobType:        jobTypeClearDeletedFeeds,
		JobDescription: "Clear update feed jobs marked for deletion.",
	}

	var (
		data []byte
		err  error
	)

	data, err = json.Marshal(newPollTrigger(24*time.Hour, time.Hour)) //nolint:mnd // Job trigger is ~every 24 hours.
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateJobFailed, err)
	}
	job.JobTrigger = data

	return job, nil
}

// executeClearDeletedFeeds runs a job that will look for update feed jobs marked for deletion and remove them from
// the scheduler queue. Jobs marked for deletion are marked by the update feed job themselves when they cannot find
// their feed in the feeds index, which indicates the feed was deleted.
func executeClearDeletedFeeds(ctx context.Context, _ *ScheduledJob) error {
	jobStateID := "clear_deleted_feeds_state"

	schedulerAPI, ok := ctx.Value(schedulerAPICtxKey).(SchedulerAPI)
	if !ok || schedulerAPI == nil {
		return errors.New("unable to get scheduler api from context")
	}

	state := &ClearDeletedFeedsState{}
	if lastState, err := schedulerAPI.GetJobState(ctx, jobStateID); err != nil {
		if !errors.Is(err, elastic.ErrNotFound) {
			return fmt.Errorf("get job state: %w", err)
		}
		state.Checkpoint = time.Time{}
	} else {
		err = json.Unmarshal(lastState.JobData, state)
		if err != nil {
			return fmt.Errorf("unmarshal job data: %w", err)
		}
	}

	// Find new feeds. We detect new feeds by those where the last_fetched value equals Unix Epoch, indicating they
	// don't have a job scheduled for updating their items.
	var (
		jobs []*ScheduledJob
		err  error
	)
	jobs, err = elastic.SearchAll[*ScheduledJob](
		ctx,
		schema.SchedulerIndexRO,
		query.Term("job_data.deleted", true),
		defaultPaginationSize,
	)
	if err != nil {
		return fmt.Errorf("search jobs: %w", err)
	}
	if len(jobs) > 0 {
		slogctx.FromCtx(ctx).DebugContext(ctx, "Found jobs that need to be deleted.",
			slog.Int("count", len(jobs)),
		)
	}
	// Update the checkpoint.
	state.Checkpoint = time.Now().UTC()
	err = schedulerAPI.UpdateJobState(ctx, jobStateID, map[string]any{
		"job_data": state,
	})
	if err != nil {
		return fmt.Errorf("update job state: %w", err)
	}

	var wg sync.WaitGroup
	// Create new feed jobs where necessary.
	for job := range slices.Values(jobs) {
		wg.Go(func() {
			if err := schedulerAPI.DeleteJob(job.JobDetail().JobKey()); err != nil {
				slogctx.FromCtx(ctx).Error("Failed to removed deleted feed job.",
					slog.String("job_id", job.JobDetail().JobKey().String()),
					slog.Any("error", err),
				)
			}
		})
	}

	wg.Wait()

	return nil
}
