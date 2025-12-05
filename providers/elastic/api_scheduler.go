// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"fmt"
	"time"

	"github.com/reugn/go-quartz/quartz"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/schema"
	"github.com/immanent-tech/foragd/scheduler/jobs"
)

// GetJobState retrieves the job state doc with the given ID from Elasticsearch.
func (a *API) GetJobState(ctx context.Context, id string) (*models.JobState, error) {
	index := schema.SchedulerIndexPrefix + schema.IndexReadSuffix

	state, err := GetDoc[string, *models.JobState](ctx, a.GetAPI(), index, id)
	if err != nil {
		return nil, fmt.Errorf("get job state: %w", err)
	}
	return state, nil
}

// UpdateJobState updates the job state doc with the given ID in Elasticsearch.
func (a *API) UpdateJobState(ctx context.Context, id string, updates map[string]any) error {
	index := schema.SchedulerIndexPrefix + schema.IndexWriteSuffix

	updates["updated_at"] = time.Now().UTC()
	if err := UpdateDoc(ctx, a.GetAPI(), index, id, updates,
		UpdateDocAsUpsert(),
		WithRefresh("true"),
	); err != nil {
		return fmt.Errorf("update job state: %w", err)
	}
	return nil
}

// ScheduleJob pushes a scheduler job into the queue.
func (a *API) ScheduleJob(ctx context.Context, id string, _ quartz.ScheduledJob, data *jobs.ScheduledJob) error {
	index := schema.SchedulerIndexPrefix + schema.IndexWriteSuffix

	if err := UpdateDoc(ctx, a.GetAPI(), index, id, map[string]any{
		"job_next_run":     data.JobNextRun,
		"job_data":         data.JobData,
		"job_trigger_type": data.JobTriggerType,
		"job_trigger":      data.JobTrigger,
		"job_type":         data.JobType,
		"updated_at":       time.Now().UTC(),
	},
		UpdateDocAsUpsert(),
		WithRefresh("true"),
	); err != nil {
		return fmt.Errorf("schedule job: %w", err)
	}

	return nil
}

// GetNextScheduledJob runs a query to find the next job to be run.
func (a *API) GetNextScheduledJob(ctx context.Context) (*jobs.ScheduledJob, error) {
	index := schema.SchedulerIndexPrefix + schema.IndexReadSuffix

	jobs, _, err := Search[*jobs.ScheduledJob](
		ctx,
		a.GetAPI(),
		index,
		query.Exists("job_type"),
		1,
	)
	if err != nil {
		return nil, fmt.Errorf("get next scheduled job: %w", err)
	}
	if len(jobs) == 0 {
		return nil, fmt.Errorf("get next scheduled job: %w", ErrNotFound)
	}
	return jobs[0], nil
}

// GetScheduledJob will return the details of the job with the given id, if it exists.
func (a *API) GetScheduledJob(ctx context.Context, id string) (*jobs.ScheduledJob, error) {
	index := schema.SchedulerIndexPrefix + schema.IndexReadSuffix

	job, err := GetDoc[string, *jobs.ScheduledJob](ctx, a.GetAPI(), index, id)
	if err != nil {
		return nil, fmt.Errorf("get scheduled job: %w", err)
	}

	return job, nil
}

// GetAllScheduledJobs returns a slice of all scheduled jobs, if any.
func (a *API) GetAllScheduledJobs(ctx context.Context) ([]jobs.ScheduledJob, error) {
	index := schema.SchedulerIndexPrefix + schema.IndexReadSuffix

	jobs, err := SearchAll[jobs.ScheduledJob](
		ctx,
		a.GetAPI(),
		index,
		query.Exists("job_type"),
		defaultPaginationSize,
	)

	if err != nil {
		return nil, fmt.Errorf("get all scheduled jobs: %w", err)
	}
	if len(jobs) == 0 {
		return nil, fmt.Errorf("get all scheduled jobs: %w", ErrNotFound)
	}

	return jobs, nil
}

// CountJobs returns a count of the scheduler jobs in the jobs index.
func (a *API) CountJobs(ctx context.Context) (int64, error) {
	index := schema.SchedulerIndexPrefix + schema.IndexReadSuffix

	count, err := Count(ctx, a.GetAPI(), index, query.Exists("job_type"))
	if err != nil {
		return 0, fmt.Errorf("count jobs: %w", err)
	}

	return count, nil
}

// RemoveAllJobs removes all scheduled jobs.
func (a *API) RemoveAllJobs(ctx context.Context) error {
	index := schema.SchedulerIndexPrefix + schema.IndexWriteSuffix

	if err := DeleteDocs(ctx, a.GetAPI(), index, query.Exists("job_type")); err != nil {
		return fmt.Errorf("remove all jobs: %w", err)
	}

	return nil
}

// RemoveJob removes a scheduled job with the given id.
func (a *API) RemoveJob(ctx context.Context, id string) error {
	index := schema.SchedulerIndexPrefix + schema.IndexWriteSuffix

	if err := DeleteDoc(ctx, a.GetAPI(), index, id); err != nil {
		return fmt.Errorf("remove job: %w", err)
	}

	return nil
}
