/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

// Package queue implements a quartz.JobQueue using Elasticsearch as the storage backend.
package queue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"
	"github.com/reugn/go-quartz/quartz"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/opensearch"
	"github.com/immanent-tech/foragd/providers/opensearch/doc"
	"github.com/immanent-tech/foragd/providers/opensearch/search"
	"github.com/immanent-tech/foragd/scheduler/jobs"
	"github.com/immanent-tech/foragd/schema"
)

const (
	// defaultRequestTimeout is the maximum time a background action can run before its context is canceled.
	defaultRequestTimeout = 10 * time.Second

	// maxPopRetries bounds the optimistic-concurrency retry loop in Pop before
	// giving up with ErrPopConflict.
	maxPopRetries = 5
)

var (
	ErrInitQueueFailed      = errors.New("could not initialize job queue")
	ErrPushJobFailed        = errors.New("push job failed")
	ErrPopJobFailed         = errors.New("pop job failed")
	ErrPopConflict          = errors.New("exhausted retries popping the head of the queue")
	ErrNoJobFound           = errors.New("no job found")
	ErrParseJobFailed       = errors.New("parsing job data failed")
	ErrGetJobFailed         = errors.New("get job failed")
	ErrRemoveJobFailed      = errors.New("remove job failed")
	ErrDeleteJobFailed      = errors.New("delete job failed")
	ErrGetJobState          = errors.New("get job state failed")
	ErrUpdateJobStateFailed = errors.New("update job state failed")
	ErrClearJobs            = errors.New("clearing jobs failed")
)

// JobQueue implements the quartz.JobQueue interface, using Elasticsearch as the persistence layer.
type JobQueue struct {
	logger *slog.Logger
	osc    *opensearch.Client
}

// Make sure out jobQueue implementation satisfies quartz.JobQueue.
var _ quartz.JobQueue = (*JobQueue)(nil)

// NewJobQueue initializes and returns an empty jobQueue.
func NewJobQueue(logger *slog.Logger, osc *opensearch.Client) (*JobQueue, error) {
	return &JobQueue{
		logger: logger,
		osc:    osc,
	}, nil
}

// Push inserts a new scheduled job to the queue. This method is also used by the Scheduler to reschedule existing jobs
// that have been dequeued for execution.
func (jq *JobQueue) Push(job quartz.ScheduledJob) error {
	ctx, cancel := jq.ctx()
	defer cancel()

	// Serialize the job into the backend storage format.
	serialized, ok := job.JobDetail().Job().(*jobs.SerializedJob)
	if !ok {
		return fmt.Errorf("%w: unsupported job type: %T", ErrPushJobFailed, job)
	}
	serialized.JobNextRun = time.Unix(0, job.NextRunTime())
	serialized.UpdatedAt = time.Now().UTC()

	// Update in backend.
	if err := doc.Index(
		ctx,
		jq.osc.Doc,
		jq.osc.IndexWriteAlias(schema.SchedulerIndex),
		serialized,
		doc.WithIndexRefresh(opensearchapi.RefreshWaitFor),
	); err != nil {
		return fmt.Errorf("%w: %w", ErrPushJobFailed, err)
	}

	return nil
}

// Pop removes and returns the next scheduled job from the queue.
func (jq *JobQueue) Pop() (quartz.ScheduledJob, error) {
	ctx, cancel := jq.ctx()
	defer cancel()

	for range maxPopRetries {
		// Get the next scheduled job.
		job, err := jq.Head()
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrPopJobFailed, err)
		}

		// Delete the job from the queue and invalidate its cache entry.
		err = jq.deleteJob(ctx, job.(*jobs.SerializedJob))
		switch {
		case err == nil:
			return job, nil
		case !isConflict(err):
			return nil, fmt.Errorf("%w: %w", ErrPopJobFailed, err)
		}
	}
	return nil, ErrPopConflict
}

// Head returns the first scheduled job without removing it from the queue.
func (jq *JobQueue) Head() (quartz.ScheduledJob, error) {
	ctx, cancel := jq.ctx()
	defer cancel()

	resp, err := search.Search(
		ctx,
		jq.osc,
		jq.osc.IndexReadAlias(schema.SchedulerIndex),
		search.WithQuery(search.MatchAllQuery()),
		search.WithSize(1),
		search.WithSort(
			search.NewSortOptions(search.WithSortField("job_next_run", search.WithFieldSortOrder(search.SortOrderAsc))),
		),
		search.WithSearchParameters(
			search.WithSeqNoPrimaryTerm(true),
		),
		search.WithTrackTotalHits(false),
		search.WithTrackScores(false),
	)
	if err != nil {
		return nil, fmt.Errorf("head: %w", err)
	}
	if len(resp.Hits.Hits) == 0 {
		return nil, ErrNoJobFound
	}

	jobs, _, err := resp.ExtractHits[*jobs.SerializedJob]()
	if err != nil {
		return nil, fmt.Errorf("head: extract hits: %w", err)
	}
	job := jobs[0]
	job.SeqNo = resp.Hits.Hits[0].SeqNo
	job.PrimaryTerm = resp.Hits.Hits[0].PrimaryTerm

	// Return the job that should run next.
	return job, nil
}

// Get returns the scheduled job with the specified key without removing it
// from the queue.
func (jq *JobQueue) Get(jobKey *quartz.JobKey) (quartz.ScheduledJob, error) {
	ctx, cancel := jq.ctx()
	defer cancel()

	// Fetch the job from the cache, loading from the backend if needed.
	job, err := doc.Get[string, *jobs.SerializedJob](ctx,
		jq.osc.Doc,
		jq.osc.IndexReadAlias(schema.SchedulerIndex),
		jobKey.String(),
	)
	switch {
	case err == nil:
		return job, nil
	case isNotFound(err):
		return nil, quartz.ErrJobNotFound
	default:
		return nil, fmt.Errorf("%w: %w", ErrGetJobFailed, err)
	}
}

// Remove removes and returns the scheduled job with the specified key.
func (jq *JobQueue) Remove(jobKey *quartz.JobKey) (quartz.ScheduledJob, error) {
	ctx, cancel := jq.ctx()
	defer cancel()

	job, err := jq.Get(jobKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRemoveJobFailed, err)
	}

	// Delete the job from the queue and invalidate its cache entry.
	err = jq.deleteJob(ctx, job.(*jobs.SerializedJob))
	switch {
	case err == nil:
		return job, nil
	case isConflict(err) || isNotFound(err):
		// Potentially concurrently removed/replaced; treat as not found.
		return nil, fmt.Errorf("%w: %w", ErrRemoveJobFailed, quartz.ErrJobNotFound)
	default:
		return nil, fmt.Errorf("%w: %w", ErrRemoveJobFailed, err)
	}
}

// ScheduledJobs returns the slice of all scheduled jobs in the queue.
func (jq *JobQueue) ScheduledJobs(matchers []quartz.Matcher[quartz.ScheduledJob]) ([]quartz.ScheduledJob, error) {
	ctx, cancel := jq.ctx()
	defer cancel()

	allJobs, err := search.SearchAll(ctx,
		jq.osc,
		jq.osc.IndexReadAlias(schema.SchedulerIndex),
		2500,
		[]search.QueryOption{search.MatchAllQuery()},
		search.WithSearchParameters(search.WithSeqNoPrimaryTerm(true)),
	)
	if err != nil {
		return nil, fmt.Errorf("get all scheduled jobs: %w", err)
	}

	serializedJobs, err := allJobs.ExtractHits[*jobs.SerializedJob]()
	if err != nil {
		return nil, fmt.Errorf("scheduled jobs: %w", err)
	}

	jobs := make([]quartz.ScheduledJob, 0, len(serializedJobs))
	// Filter jobs that to those that match given matchers.
	for job := range slices.Values(serializedJobs) {
		if isMatch(job, matchers) {
			jobs = append(jobs, job)
		}
	}

	return jobs, nil
}

// Size returns the size of the job queue.
func (jq *JobQueue) Size() (int, error) {
	ctx, cancel := jq.ctx()
	defer cancel()

	count, err := search.Count(
		ctx,
		jq.osc,
		jq.osc.IndexReadAlias(schema.SchedulerIndex),
		search.WithCountQuery(search.MatchAllQuery()),
	)
	if err != nil {
		return 0, fmt.Errorf("count jobs: %w", err)
	}

	return int(count), nil
}

// Clear clears the job queue.
func (jq *JobQueue) Clear() error {
	ctx, cancel := jq.ctx()
	defer cancel()

	if err := doc.DeleteByQuery(
		ctx,
		jq.osc,
		jq.osc.IndexWriteAlias(schema.SchedulerIndex),
		doc.WithDeleteByQuery(search.MatchAllQuery()),
	); err != nil {
		return fmt.Errorf("%w: delete docs: %w", ErrClearJobs, err)
	}

	jq.logger.Debug("Cleared job queue.")
	return nil
}

func (jq *JobQueue) deleteJob(ctx context.Context, job *jobs.SerializedJob) error {
	if err := doc.Delete(
		ctx,
		jq.osc.Doc,
		jq.osc.IndexWriteAlias(schema.SchedulerIndex),
		job.GetID(),
		doc.WithDeleteRefresh(opensearchapi.RefreshWaitFor),
	); err != nil {
		return fmt.Errorf("%w: %w", ErrDeleteJobFailed, err)
	}

	return nil
}

func (q *JobQueue) ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), defaultRequestTimeout)
}

func isMatch(job quartz.ScheduledJob, matchers []quartz.Matcher[quartz.ScheduledJob]) bool {
	for matcher := range slices.Values(matchers) {
		// require all matchers to match the job
		if !matcher.IsMatch(job) {
			return false
		}
	}

	return true
}

type scheduledJob struct {
	*models.DocMetadata

	jobDetail   *quartz.JobDetail
	trigger     quartz.Trigger
	nextRunTime int64
}

var _ quartz.ScheduledJob = (*scheduledJob)(nil)

func (job *scheduledJob) JobDetail() *quartz.JobDetail { return job.jobDetail }
func (job *scheduledJob) Trigger() quartz.Trigger      { return job.trigger }
func (job *scheduledJob) NextRunTime() int64           { return job.nextRunTime }

func isNotFound(err error) bool {
	if apiErr, ok := errors.AsType[*models.APIError](err); ok {
		return apiErr.StatusCode == http.StatusNotFound
	}
	return false
}

func isConflict(err error) bool {
	if apiErr, ok := errors.AsType[*models.APIError](err); ok {
		return apiErr.StatusCode == http.StatusConflict
	}
	return false
}
