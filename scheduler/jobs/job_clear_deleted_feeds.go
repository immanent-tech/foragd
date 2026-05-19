// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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

	start := time.Now()

	slogctx.FromCtx(ctx).DebugContext(ctx, "Clearing deleted feeds.",
		slog.Time("since", data.Checkpoint),
	)

	schedulerAPI, ok := ctx.Value(schedulerAPICtxKey).(SchedulerAPI)
	if !ok || schedulerAPI == nil {
		return errors.New("unable to get scheduler api from context")
	}

	// Find new feeds. We detect new feeds by those where the last_fetched value equals Unix Epoch, indicating they
	// don't have a job scheduled for updating their items.
	var (
		jobs []*SerializedJob
	)
	jobs, err = elastic.SearchAll[*SerializedJob](
		ctx,
		schema.SchedulerIndexRO(),
		query.Term("job_data.deleted", true),
		5000,
	)
	if err != nil {
		return fmt.Errorf("search jobs: %w", err)
	}
	if len(jobs) > 0 {
		slogctx.FromCtx(ctx).Info("Found feed jobs that need to be deleted.",
			slog.Int("count", len(jobs)),
		)
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
			} else {
				slogctx.FromCtx(ctx).Info("Removed feed job marked for deletion.",
					slog.String("job_id", job.JobDetail().JobKey().String()),
				)
			}
		})
	}

	wg.Wait()

	// Update the job data (checkpoint).
	if err := job.JobData.MergeClearDeletedFeedsJob(
		ClearDeletedFeedsJob{Checkpoint: time.Now().UTC()},
	); err != nil {
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

	slogctx.FromCtx(ctx).Debug("Finished clearing deleted feeds.",
		slog.Duration("took", time.Since(start)))

	return nil
}
