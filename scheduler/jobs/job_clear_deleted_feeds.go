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
	"time"

	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
)

// NewClearDeletedFeedsJob creates a job for clearing deleted feeds.
func NewClearDeletedFeedsJob() (*SerializedJob, error) {
	job := &SerializedJob{
		CreatedAt:      time.Now().UTC(),
		JobDescription: new("Clear deleted feeds."),
		JobKey:         quartz.NewJobKey(string(JobTypeClearDeletedFeeds)).String(),
		JobType:        JobTypeClearDeletedFeeds,
		JobNextRun:     models.UnixEpoch,
		JobTriggerType: TriggerTypePoll,
	}

	if err := job.JobData.FromClearDeletedFeedsJob(
		ClearDeletedFeedsJob{Checkpoint: models.UnixEpoch},
	); err != nil {
		return nil, fmt.Errorf("create job data: %w", err)
	}

	if err := job.JobTrigger.FromPollTrigger(*NewPollTrigger(24*time.Hour, 5*time.Minute)); err != nil {
		return nil, fmt.Errorf("create trigger: %w", err)
	}

	return job, nil
}

// ExecuteClearDeletedFeeds runs a job that will look for update feed jobs marked for deletion and remove them from
// the scheduler queue. Jobs marked for deletion are marked by the update feed job themselves when they cannot find
// their feed in the feeds index, which indicates the feed was deleted.
func ExecuteClearDeletedFeeds(ctx context.Context, job *SerializedJob) error {
	data, err := job.JobData.AsDeleteExpiredSessionsJob()
	if err != nil {
		return fmt.Errorf("unable to unmarshal job data: %w", err)
	}

	services := ServicesFromCtx(ctx)
	if services == nil {
		return errors.New("no services in context")
	}

	start := time.Now()

	slogctx.FromCtx(ctx).DebugContext(ctx, "Clearing deleted feeds.",
		slog.Time("since", data.Checkpoint),
	)

	if err := services.Scheduler.ClearDeletedFeedJobs(ctx); err != nil {
		return fmt.Errorf("clear deleted feed jobs: %w", err)
	}

	// Update the job data (checkpoint).
	if err := job.JobData.MergeClearDeletedFeedsJob(
		ClearDeletedFeedsJob{Checkpoint: time.Now().UTC()},
	); err != nil {
		return fmt.Errorf("update job data: %w", err)
	}
	if err := services.Scheduler.UpdateJob(ctx, job); err != nil {
		return fmt.Errorf("update job: %w", err)
	}

	slogctx.FromCtx(ctx).Debug("Finished clearing deleted feeds.",
		slog.Duration("took", time.Since(start)))

	return nil
}
