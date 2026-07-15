// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package queue implements a quartz.JobQueue using Elasticsearch as the storage backend.
package queue

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/maypok86/otter/v2"
	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/logging"

	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/scheduler/jobs"
)

const (
	// defaultRequestTimeout is the maximum time a background action can run before its context is canceled.
	defaultRequestTimeout   = 10 * time.Second
	gracefulShutdownTimeout = 30 * time.Second
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
	cache  *otter.Cache[*quartz.JobKey, *jobs.SerializedJob]
	loader otter.LoaderFunc[*quartz.JobKey, *jobs.SerializedJob]
}

// Make sure out jobQueue implementation satisfies quartz.JobQueue.
var _ quartz.JobQueue = (*JobQueue)(nil)

// NewJobQueue initializes and returns an empty jobQueue.
func NewJobQueue(ctx context.Context) (*JobQueue, error) {
	queue := &JobQueue{
		logger: slogctx.FromCtx(ctx),
		cache: otter.Must(&otter.Options[*quartz.JobKey, *jobs.SerializedJob]{
			MaximumSize: 10_000,
			OnAtomicDeletion: func(entry otter.DeletionEvent[*quartz.JobKey, *jobs.SerializedJob]) {
				ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
				defer cancel()

				// Update in backend.
				if err := bulk.AddAction(ctx,
					bulk.NewAction(
						entry.Value,
						bulk.AsOperation[string](bulk.OpDelete),
						bulk.ToIndex[string](schema.SchedulerIndexRW()),
					),
				); err != nil {
					slogctx.FromCtx(ctx).Error("Unable to delete scheduled job.",
						slog.String("job_key", entry.Key.String()),
						slog.Any("error", err))
					return
				}

				// if err := elastic.DeleteDoc(ctx, schema.SchedulerIndexRW(), entry.Key.String()); err != nil {
				// 	slogctx.FromCtx(ctx).Error("Unable to delete scheduled job.",
				// 		slog.String("job_key", entry.Key.String()),
				// 		slog.Any("error", err))
				// 	return
				// }
				slogctx.FromCtx(ctx).Debug("Scheduled job was deleted.",
					slog.String("job_key", entry.Key.String()),
					slog.String("reason", entry.Cause.String()),
				)
			},
		}),
		loader: func(ctx context.Context, key *quartz.JobKey) (*jobs.SerializedJob, error) {
			if err := bulk.Flush(ctx); err != nil {
				return nil, fmt.Errorf("%w: %w", ErrGetJobFailed, err)
			}
			getCtx, cancel := context.WithTimeout(ctx, defaultRequestTimeout)
			defer cancel()
			job, err := elastic.GetDoc[string, *jobs.SerializedJob](
				getCtx,
				schema.SchedulerIndexRO(),
				key.String(),
			)
			if err != nil {
				return nil, fmt.Errorf("%w: %w: %w", ErrGetJobFailed, otter.ErrNotFound, err)
			}

			slogctx.FromCtx(ctx).Debug("Retrieved job from backend.",
				slog.String("job_key", job.JobDetail().JobKey().String()),
			)

			return job, nil
		},
	}

	// Get all existing jobs from the backend.
	const defaultPaginationSize = 5000
	jobs, err := elastic.SearchAll[*jobs.SerializedJob](
		ctx,
		schema.SchedulerIndexRO(),
		query.MatchAll(),
		defaultPaginationSize,
	)
	if err != nil {
		return nil, fmt.Errorf("get all scheduled jobs: %w", err)
	}

	// Load all jobs into cache.
	for job := range slices.Values(jobs) {
		queue.cache.Set(job.JobDetail().JobKey(), job)
	}

	slogctx.FromCtx(ctx).Info("Job queue started.",
		slog.Time("start_time", time.Now().UTC()),
		slog.Int("job_count", queue.cache.EstimatedSize()))

	return queue, nil
}

// Push inserts a new scheduled job to the queue. This method is also used by the Scheduler to reschedule existing jobs
// that have been dequeued for execution.
func (jq *JobQueue) Push(job quartz.ScheduledJob) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	// Serialize the job into the backend storage format.
	serialized, ok := job.JobDetail().Job().(*jobs.SerializedJob)
	if !ok {
		return fmt.Errorf("%w: unsupported job type: %T", ErrPushJobFailed, job)
	}
	serialized.JobNextRun = time.Unix(0, job.NextRunTime())
	serialized.UpdatedAt = time.Now().UTC()

	// Update in backend.
	if err := bulk.AddAction(ctx,
		bulk.NewAction(
			serialized,
			bulk.AsOperation[string](bulk.OpIndex),
			bulk.ToIndex[string](schema.SchedulerIndexRW()),
		),
	); err != nil {
		return fmt.Errorf("%w: %w", ErrPushJobFailed, err)
	}

	// Update in cache.
	jq.cache.Set(job.JobDetail().JobKey(), serialized)

	return nil
}

// Pop removes and returns the next scheduled job from the queue.
func (jq *JobQueue) Pop() (quartz.ScheduledJob, error) {
	// Get the next scheduled job.
	job, err := jq.Head()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPopJobFailed, err)
	}

	// Invalidate it in the cache so it will be fetched from the backend next time.
	jq.cache.Invalidate(job.JobDetail().JobKey())

	return job, nil
}

// Head returns the first scheduled job without removing it from the queue.
func (jq *JobQueue) Head() (quartz.ScheduledJob, error) {
	// Get all jobs from the cache.
	allJobs := slices.Collect(jq.cache.Values())
	if len(allJobs) == 0 {
		return nil, fmt.Errorf("head: %w", quartz.ErrQueueEmpty)
	}

	// Sort jobs in ascending order by next run time.
	slices.SortFunc(allJobs, func(a, b *jobs.SerializedJob) int {
		return cmp.Compare(a.NextRunTime(), b.NextRunTime())
	})

	// Return the job that should run next.
	return allJobs[0], nil
}

// Get returns the scheduled job with the specified key without removing it
// from the queue.
func (jq *JobQueue) Get(jobKey *quartz.JobKey) (quartz.ScheduledJob, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	// Fetch the job from the cache, loading from the backend if needed.
	job, err := jq.cache.Get(ctx, jobKey, jq.loader)
	if err != nil {
		if errors.Is(err, otter.ErrNotFound) {
			return nil, fmt.Errorf("%w: %w: %w", ErrGetJobFailed, quartz.ErrJobNotFound, err)
		}
		return nil, fmt.Errorf("%w: %w", ErrGetJobFailed, err)
	}

	return job, nil
}

// Remove removes and returns the scheduled job with the specified key.
func (jq *JobQueue) Remove(jobKey *quartz.JobKey) (quartz.ScheduledJob, error) {
	// Get the job.
	job, err := jq.Get(jobKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRemoveJobFailed, err)
	}

	// Invalidate the cache entry for the job.
	jq.cache.Invalidate(jobKey)

	jq.logger.Debug("Removed job from queue.",
		slog.String("job_key", job.JobDetail().JobKey().String()))

	return job, nil
}

// ScheduledJobs returns the slice of all scheduled jobs in the queue.
func (jq *JobQueue) ScheduledJobs(matchers []quartz.Matcher[quartz.ScheduledJob]) ([]quartz.ScheduledJob, error) {
	jobs := make([]quartz.ScheduledJob, 0)
	// Filter jobs that to those that match given matchers.
	for job := range slices.Values(slices.Collect(jq.cache.Values())) {
		if isMatch(job, matchers) {
			jobs = append(jobs, job)
		}
	}
	return jobs, nil
}

// Size returns the size of the job queue.
func (jq *JobQueue) Size() (int, error) {
	return len(slices.Collect(jq.cache.Values())), nil
}

// Clear clears the job queue.
func (jq *JobQueue) Clear() error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	// Invalidate all cache entries.
	jq.cache.InvalidateAll()

	jq.logger.Log(ctx, logging.LevelTrace, "Cleared job queue.")
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
