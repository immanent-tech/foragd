// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package queue implements a quartz.JobQueue using Elasticsearch as the storage backend.
package queue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/logging"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/scheduler/jobs"
)

const (
	// defaultRequestTimeout is the maximum time a background action can run before its context is cancelled.
	defaultRequestTimeout = 5 * time.Second
	// defaultPaginationSize is the default number of docs to fetch when paginating through results from elasticsearch.
)

var (
	ErrInitQueueFailed      = errors.New("could not initialize job queue")
	ErrPushJobFailed        = errors.New("push job failed")
	ErrPopJobFailed         = errors.New("pop job failed")
	ErrNoJobFound           = errors.New("no job found")
	ErrParseJobFailed       = errors.New("parsing job data failed")
	ErrGetJobFailed         = errors.New("get job failed")
	ErrRemoveJobFailed      = errors.New("remove job failed")
	ErrDeleteJobFailed      = errors.New("delete job failed")
	ErrGetJobState          = errors.New("get job state failed")
	ErrUpdateJobStateFailed = errors.New("update job state failed")
	ErrClearJobs            = errors.New("clearing jobs failed")
)

// JobQueue implements the quartz.JobQueue interface, using Elasticsearch as the
// persistence layer.
type JobQueue struct {
	logger *slog.Logger
}

// Make sure out jobQueue implementation satisfies quartz.JobQueue.
var _ quartz.JobQueue = (*JobQueue)(nil)

// NewJobQueue initializes and returns an empty jobQueue.
func NewJobQueue(ctx context.Context) (*JobQueue, error) {
	return &JobQueue{logger: slogctx.FromCtx(ctx)}, nil
}

// Push inserts a new scheduled job to the queue. This method is also used by the Scheduler to reschedule existing jobs
// that have been dequeued for execution.
func (jq *JobQueue) Push(job quartz.ScheduledJob) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	serialized, ok := job.JobDetail().Job().(*jobs.SerializedJob)
	if !ok {
		return fmt.Errorf("%w: unsupported job type: %T", ErrPushJobFailed, job)
	}

	serialized.JobNextRun = time.Unix(0, job.NextRunTime())
	serialized.UpdatedAt = time.Now().UTC()

	if err := elastic.UpdateDoc(
		ctx,
		schema.SchedulerIndexRW(),
		job.JobDetail().JobKey().String(),
		serialized,
		elastic.WithDocAsUpsert(true),
		elastic.WithRefresh(true),
	); err != nil {
		return fmt.Errorf("%w: %w", ErrPushJobFailed, err)
	}

	jq.logger.Log(ctx, logging.LevelTrace, "Pushed job to queue.",
		slog.Group("job",
			slog.String("id", job.JobDetail().JobKey().String()),
		),
	)

	return nil
}

// Pop removes and returns the next scheduled job from the queue.
func (jq *JobQueue) Pop() (quartz.ScheduledJob, error) {
	job, err := jq.Head()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPopJobFailed, err)
	}
	id := job.JobDetail().JobKey().String()
	err = jq.delete(id)
	if err != nil {
		return nil, errors.Join(ErrPopJobFailed, err)
	}
	jq.logger.Log(context.Background(), logging.LevelTrace, "Popped job from queue.",
		slog.Group("job",
			slog.String("id", job.JobDetail().JobKey().String()),
		),
	)
	return job, nil
}

// Head returns the first scheduled job without removing it from the queue.
func (jq *JobQueue) Head() (quartz.ScheduledJob, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	// Find the next job
	resp, err := elastic.Search[*jobs.SerializedJob](
		ctx,
		schema.SchedulerIndexRO(),
		elastic.WithQueryOptions[*elastic.SearchRequest](query.MatchAll()),
		elastic.WithSort(&jobSorting{JobNextRun: "asc"}),
		elastic.WithSize(1),
	)
	if err != nil {
		return nil, fmt.Errorf("head: %w", err)
	}
	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("head: %w", quartz.ErrQueueEmpty)
	}
	return resp.Results[0], nil
}

// Get returns the scheduled job with the specified key without removing it
// from the queue.
func (jq *JobQueue) Get(jobKey *quartz.JobKey) (quartz.ScheduledJob, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	job, err := elastic.GetDoc[string, *jobs.SerializedJob](
		ctx,
		schema.SchedulerIndexRO(),
		jobKey.String(),
	)
	if err != nil {
		if errors.Is(err, elastic.ErrNotFound) {
			return nil, quartz.ErrJobNotFound
		}
		return nil, fmt.Errorf("%w: %w", ErrGetJobFailed, err)
	}

	return job, nil
}

// Remove removes and returns the scheduled job with the specified key.
func (jq *JobQueue) Remove(jobKey *quartz.JobKey) (quartz.ScheduledJob, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	job, err := jq.Get(jobKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRemoveJobFailed, err)
	}
	id := jobKey.String()
	err = jq.delete(id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRemoveJobFailed, err)
	}
	jq.logger.Log(ctx, logging.LevelTrace, "Job removed.",
		slog.String("job", job.JobDetail().Job().Description()))

	return job, nil
}

// ScheduledJobs returns the slice of all scheduled jobs in the queue.
func (jq *JobQueue) ScheduledJobs(matchers []quartz.Matcher[quartz.ScheduledJob]) ([]quartz.ScheduledJob, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	allJobs, err := elastic.SearchAll[*jobs.SerializedJob](
		ctx,
		schema.SchedulerIndexRO(),
		query.MatchAll(),
		1000,
	)
	if err != nil {
		return nil, fmt.Errorf("get all scheduled jobs: %w", err)
	}
	if len(allJobs) == 0 {
		return nil, fmt.Errorf("get all scheduled jobs: %w", quartz.ErrJobNotFound)
	}

	jobs := make([]quartz.ScheduledJob, 0, len(allJobs))
	// Filter jobs that to those that match given matchers.
	for _, job := range allJobs {
		if isMatch(job, matchers) {
			jobs = append(jobs, job)
		}
	}
	return jobs, nil
}

// Size returns the size of the job queue.
func (jq *JobQueue) Size() (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	count, err := elastic.Count(ctx, schema.SchedulerIndexRO(), query.MatchAll())
	if err != nil {
		return 0, fmt.Errorf("count jobs: %w", err)
	}

	return int(count), nil
}

// Clear clears the job queue.
func (jq *JobQueue) Clear() error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	if err := elastic.DeleteDocs(ctx, schema.SchedulerIndexRW(), query.MatchAll()); err != nil {
		return fmt.Errorf("%w: %w", ErrClearJobs, err)
	}

	jq.logger.Log(ctx, logging.LevelTrace, "Cleared job queue.")
	return nil
}

// delete removes the job doc from Elasticsearch.
func (jq *JobQueue) delete(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	if err := elastic.DeleteDoc(ctx, schema.SchedulerIndexRW(), id); err != nil {
		return fmt.Errorf("%w: %w", ErrDeleteJobFailed, err)
	}

	return nil
}

func isMatch(job quartz.ScheduledJob, matchers []quartz.Matcher[quartz.ScheduledJob]) bool {
	for _, matcher := range matchers {
		// require all matchers to match the job
		if !matcher.IsMatch(job) {
			return false
		}
	}

	return true
}

type jobSorting struct {
	JobNextRun string `json:"job_next_run"`
}

// SortCombinationsCaster is required to allow ItemSorting to be used as Elasticsearch sort values.
func (s *jobSorting) SortCombinationsCaster() *types.SortCombinations {
	c := types.SortCombinations(s)
	return &c
}
