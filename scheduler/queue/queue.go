// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package queue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/logging"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/scheduler/jobs"
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

type storeAPI interface {
	ScheduleJob(ctx context.Context, id string, _ quartz.ScheduledJob, data *jobs.ScheduledJob) error
	GetNextScheduledJob(ctx context.Context) (*jobs.ScheduledJob, error)
	GetScheduledJob(ctx context.Context, id string) (*jobs.ScheduledJob, error)
	GetAllScheduledJobs(ctx context.Context) ([]jobs.ScheduledJob, error)
	CountJobs(ctx context.Context) (int64, error)
	RemoveAllJobs(ctx context.Context) error
	RemoveJob(ctx context.Context, id string) error
}

// JobQueue implements the quartz.JobQueue interface, using Elasticsearch as the
// persistence layer.
type JobQueue struct {
	logger   *slog.Logger
	storeAPI storeAPI
}

// NewJobQueue initializes and returns an empty jobQueue.
func NewJobQueue(ctx context.Context, storeAPI storeAPI) (*JobQueue, error) {
	return &JobQueue{
		logger:   slogctx.FromCtx(ctx),
		storeAPI: storeAPI,
	}, nil
}

// Push inserts a new scheduled job to the queue.
// This method is also used by the Scheduler to reschedule existing jobs that
// have been dequeued for execution.
func (jq *JobQueue) Push(job quartz.ScheduledJob) error {
	data, err := jobs.MarshalJob(job)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPushJobFailed, err)
	}

	ctx := context.Background()

	err = jq.storeAPI.ScheduleJob(ctx, jobKeyToDocID(job.JobDetail().JobKey().String()), job, data)
	if err != nil {
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
	ctx := context.Background()

	job, err := jq.storeAPI.GetNextScheduledJob(ctx)
	if err != nil {
		if models.HTTPStatus(err) == http.StatusNotFound {
			return nil, fmt.Errorf("head: %w", quartz.ErrQueueEmpty)
		}
		return nil, fmt.Errorf("head: %w", err)
	}
	return job, nil
}

// Get returns the scheduled job with the specified key without removing it
// from the queue.
func (jq *JobQueue) Get(jobKey *quartz.JobKey) (quartz.ScheduledJob, error) {
	ctx := context.Background()

	job, err := jq.storeAPI.GetScheduledJob(ctx, jobKeyToDocID(jobKey.String()))
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
	job, err := jq.Get(jobKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRemoveJobFailed, err)
	}
	id := jobKeyToDocID(jobKey.String())
	err = jq.delete(id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRemoveJobFailed, err)
	}
	jq.logger.Log(context.Background(), logging.LevelTrace, "Job removed.",
		slog.String("job", job.JobDetail().Job().Description()))

	return job, nil
}

// ScheduledJobs returns the slice of all scheduled jobs in the queue.
func (jq *JobQueue) ScheduledJobs(matchers []quartz.Matcher[quartz.ScheduledJob]) ([]quartz.ScheduledJob, error) {
	jobs := make([]quartz.ScheduledJob, 0)

	ctx := context.Background()

	allJobs, err := jq.storeAPI.GetAllScheduledJobs(ctx)
	if err != nil {
		if errors.Is(err, elastic.ErrNotFound) {
			return nil, quartz.ErrJobNotFound
		}
		return nil, fmt.Errorf("%w: %w", ErrGetJobFailed, err)
	}

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
	ctx := context.Background()

	count, err := jq.storeAPI.CountJobs(ctx)
	if err != nil {
		return 0, fmt.Errorf("could not get size of scheduled jobs queue: %w", err)
	}
	return int(count), nil
}

// Clear clears the job queue.
func (jq *JobQueue) Clear() error {
	ctx := context.Background()

	err := jq.storeAPI.RemoveAllJobs(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrClearJobs, err)
	}

	jq.logger.Log(ctx, logging.LevelTrace, "Cleared job queue.")
	return nil
}

// delete removes the job doc from Elasticsearch.
func (jq *JobQueue) delete(id string) error {
	ctx := context.Background()

	err := jq.storeAPI.RemoveJob(ctx, id)
	if err != nil {
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
