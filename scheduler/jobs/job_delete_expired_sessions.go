// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

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
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/service"
)

// NewDeleteExpiredSessionsJob creates a job for deleting expired user sessions.
func NewDeleteExpiredSessionsJob() (*SerializedJob, error) {
	job := &SerializedJob{
		CreatedAt:      time.Now().UTC(),
		JobDescription: new("Clear expired user sessions."),
		JobKey:         quartz.NewJobKey(string(JobTypeDeleteExpiredSessions)).String(),
		JobType:        JobTypeDeleteExpiredSessions,
		JobNextRun:     models.UnixEpoch,
		JobTriggerType: TriggerTypePoll,
	}

	if err := job.JobData.FromDeleteExpiredSessionsJob(
		DeleteExpiredSessionsJob{Checkpoint: models.UnixEpoch},
	); err != nil {
		return nil, fmt.Errorf("create job data: %w", err)
	}

	if err := job.JobTrigger.FromPollTrigger(*NewPollTrigger(24*time.Hour, 5*time.Minute)); err != nil {
		return nil, fmt.Errorf("create trigger: %w", err)
	}

	return job, nil
}

func ExecuteDeleteExpiredSessions(ctx context.Context, job *SerializedJob) error {
	data, err := job.JobData.AsDeleteExpiredSessionsJob()
	if err != nil {
		return fmt.Errorf("unable to unmarshal job data: %w", err)
	}

	elasticSvc := ElasticFromCtx(ctx)
	if elasticSvc == nil {
		return errors.New("cannot execute: no elastic service in context")
	}

	start := time.Now()

	slogctx.FromCtx(ctx).DebugContext(ctx, "Deleting expired sessions.",
		slog.Time("since", data.Checkpoint),
	)

	// Delete all sessions with an expiry older than now.
	if err := elastic.DeleteDocs(
		ctx,
		elasticSvc.GetIndexRW(service.SessionsIndex),
		query.Before("expiry", time.Now().UTC()),
	); err != nil {
		return fmt.Errorf("delete docs: %w", err)
	}

	// Update the job data (checkpoint).
	if err := job.JobData.MergeDeleteExpiredSessionsJob(
		DeleteExpiredSessionsJob{Checkpoint: time.Now().UTC()},
	); err != nil {
		return fmt.Errorf("update job data: %w", err)
	}
	if err := elastic.UpdateDoc(
		ctx,
		elasticSvc.GetIndexRW(service.ScheduleIndex),
		job.JobDetail().JobKey().String(),
		job,
		elastic.WithDocAsUpsert(true),
	); err != nil {
		return fmt.Errorf("update job: %w", err)
	}

	slogctx.FromCtx(ctx).Debug("Finished delete expired sessions job.",
		slog.Duration("took", time.Since(start)))

	return nil
}
