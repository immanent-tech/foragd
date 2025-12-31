// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package queue implements a quartz.JobQueue using Elasticsearch as the storage backend.
package queue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/logging"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/schema"
	"github.com/immanent-tech/foragd/scheduler/jobs"
)

const (
	// defaultRequestTimeout is the maximum time a background action can run before its context is cancelled.
	defaultRequestTimeout = 5 * time.Second
	// defaultPaginationSize is the default number of docs to fetch when paginating through results from elasticsearch.
	defaultPaginationSize = 5000
)

// Make sure out jobQueue implementation satisfies quartz.JobQueue.
var _ quartz.JobQueue = (*JobQueue)(nil)

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

// NewJobQueue initializes and returns an empty jobQueue.
func NewJobQueue(ctx context.Context) (*JobQueue, error) {
	return &JobQueue{logger: slogctx.FromCtx(ctx)}, nil
}

// Push inserts a new scheduled job to the queue.
// This method is also used by the Scheduler to reschedule existing jobs that
// have been dequeued for execution.
func (jq *JobQueue) Push(job quartz.ScheduledJob) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	data, err := jobs.MarshalJob(job)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPushJobFailed, err)
	}

	if err := elastic.UpdateDoc(ctx, schema.SchedulerIndexRW, jobKeyToDocID(job.JobDetail().JobKey().String()), map[string]any{
		"job_next_run":     data.JobNextRun,
		"job_data":         data.JobData,
		"job_trigger_type": data.JobTriggerType,
		"job_trigger":      data.JobTrigger,
		"job_type":         data.JobType,
		"updated_at":       time.Now().UTC(),
	},
		elastic.UpdateDocAsUpsert(),
		elastic.WithRefresh("true"),
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
	id := jobKeyToDocID(job.JobDetail().JobKey().String())
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
	jobs, _, err := elastic.Search[*jobs.ScheduledJob](
		ctx,
		schema.SchedulerIndexRO,
		query.Exists("job_type"),
		1,
		// elastic.WithSortOptions[*search.Search, elastic.SearchRequest](&jobSorting{JobNextRun: "desc"}),
	)
	if err != nil {
		return nil, fmt.Errorf("head: %w", err)
	}
	if len(jobs) == 0 {
		return nil, fmt.Errorf("head: %w", quartz.ErrQueueEmpty)
	}

	return jobs[0], nil
}

// Get returns the scheduled job with the specified key without removing it
// from the queue.
func (jq *JobQueue) Get(jobKey *quartz.JobKey) (quartz.ScheduledJob, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	job, err := elastic.GetDoc[string, *jobs.ScheduledJob](ctx, schema.SchedulerIndexRO, jobKeyToDocID(jobKey.String()))
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
	id := jobKeyToDocID(jobKey.String())
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

	allJobs, err := elastic.SearchAll[jobs.ScheduledJob](
		ctx,
		schema.SchedulerIndexRO,
		query.Exists("job_type"),
		defaultPaginationSize,
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
		if isMatch(&job, matchers) {
			jobs = append(jobs, &job)
		}
	}
	return jobs, nil
}

// Size returns the size of the job queue.
func (jq *JobQueue) Size() (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	count, err := elastic.Count(ctx, schema.SchedulerIndexRO, query.Exists("job_type"))
	if err != nil {
		return 0, fmt.Errorf("count jobs: %w", err)
	}

	return int(count), nil
}

// Clear clears the job queue.
func (jq *JobQueue) Clear() error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	if err := elastic.DeleteDocs(ctx, schema.SchedulerIndexRW, query.Exists("job_type")); err != nil {
		return fmt.Errorf("%w: %w", ErrClearJobs, err)
	}

	jq.logger.Log(ctx, logging.LevelTrace, "Cleared job queue.")
	return nil
}

// delete removes the job doc from Elasticsearch.
func (jq *JobQueue) delete(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	if err := elastic.DeleteDoc(ctx, schema.SchedulerIndexRW, id); err != nil {
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

func jobKeyToDocID(jobkey string) string {
	parts := strings.Split(jobkey, "::")
	return parts[1]
}

type jobSorting struct {
	JobNextRun string `json:"job_next_run"`
}

// SortCombinationsCaster is required to allow ItemSorting to be used as Elasticsearch sort values.
func (s *jobSorting) SortCombinationsCaster() *types.SortCombinations {
	c := types.SortCombinations(s)
	return &c
}
