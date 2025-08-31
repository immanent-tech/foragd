// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/sortorder"
	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-feed-me/logging"
	"github.com/immanent-tech/go-feed-me/providers/elastic"
	"github.com/immanent-tech/go-feed-me/providers/elastic/query"
	"github.com/immanent-tech/go-feed-me/providers/elastic/schema"
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
//
//nolint:containedctx // Interface cannot be made context-aware due to underlying package limitations.
type JobQueue struct {
	client *elastic.API
	index  string
	ctx    context.Context
}

// NewJobQueue initializes and returns an empty jobQueue.
func NewJobQueue(ctx context.Context, client *elastic.API) (*JobQueue, error) {
	return &JobQueue{
		ctx:    ctx,
		client: client,
		index:  schema.SchedulerJobsPrefix,
	}, nil
}

// Push inserts a new scheduled job to the queue.
// This method is also used by the Scheduler to reschedule existing jobs that
// have been dequeued for execution.
func (jq *JobQueue) Push(job quartz.ScheduledJob) error {
	data, err := MarshalJob(job)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPushJobFailed, err)
	}

	id := jobKeyToDocID(job.JobDetail().JobKey().String())

	err = elastic.UpdateDoc(jq.ctx, jq.client.GetAPI(), jq.index, id, map[string]any{
		"job_next_run":     data.JobNextRun,
		"job_data":         data.JobData,
		"job_trigger_type": data.JobTriggerType,
		"job_trigger":      data.JobTrigger,
		"job_type":         data.JobType,
		"updated_at":       time.Now().UTC(),
	},
		elastic.UpdateDocAsUpsert(),
		elastic.WithRefresh("true"),
	)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPushJobFailed, err)
	}

	slogctx.FromCtx(jq.ctx).Log(jq.ctx, logging.LevelTrace, "Pushed job to queue.",
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
	slogctx.FromCtx(jq.ctx).Log(jq.ctx, logging.LevelTrace, "Popped job from queue.",
		slog.Group("job",
			slog.String("id", job.JobDetail().JobKey().String()),
		),
	)
	return job, nil
}

// Head returns the first scheduled job without removing it from the queue.
func (jq *JobQueue) Head() (quartz.ScheduledJob, error) {
	jobs, _, err := elastic.Search[*ScheduledJob](jq.ctx, jq.client.GetAPI(), jq.index, query.MatchAll(), 1,
		elastic.WithSortOptions[*search.Search, elastic.SearchRequest](&jobSorting{}),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrGetJobFailed, err)
	}
	if len(jobs) == 0 {
		return nil, quartz.ErrQueueEmpty
	}
	return jobs[0], nil
}

// Get returns the scheduled job with the specified key without removing it
// from the queue.
func (jq *JobQueue) Get(jobKey *quartz.JobKey) (quartz.ScheduledJob, error) {
	id := jobKeyToDocID(jobKey.String())
	job, err := elastic.GetDoc[string, ScheduledJob](jq.ctx, jq.client.GetAPI(), jq.index, id)
	if err != nil {
		if errors.Is(err, elastic.ErrNotFound) {
			return nil, quartz.ErrJobNotFound
		}
		return nil, fmt.Errorf("%w: %w", ErrGetJobFailed, err)
	}
	return &job, nil
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
	slogctx.FromCtx(jq.ctx).DebugContext(jq.ctx, "Job removed.",
		slog.String("job", job.JobDetail().Job().Description()))

	return job, nil
}

// ScheduledJobs returns the slice of all scheduled jobs in the queue.
func (jq *JobQueue) ScheduledJobs(matchers []quartz.Matcher[quartz.ScheduledJob]) ([]quartz.ScheduledJob, error) {
	jobs := make([]quartz.ScheduledJob, 0)
	allJobs, err := elastic.SearchAll[ScheduledJob](jq.ctx, jq.client.GetAPI(), jq.index, query.MatchAll(), 1000)
	if err != nil {
		return nil, fmt.Errorf("failed to get scheduled jobs: %w", err)
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
	count, err := elastic.Count(jq.ctx, jq.client.GetAPI(), jq.index, query.MatchAll())
	if err != nil {
		return 0, fmt.Errorf("could not get size of scheduled jobs queue: %w", err)
	}
	return int(count), nil
}

// Clear clears the job queue.
func (jq *JobQueue) Clear() error {
	err := elastic.DeleteDocs(jq.ctx, jq.client.GetAPI(), jq.index, query.MatchAll())
	if err != nil {
		return fmt.Errorf("%w: %w", ErrClearJobs, err)
	}
	slogctx.FromCtx(jq.ctx).DebugContext(jq.ctx, "Cleared job queue.")
	return nil
}

// delete removes the job doc from Elasticsearch.
func (jq *JobQueue) delete(id string) error {
	err := elastic.DeleteDoc(jq.ctx, jq.client.GetAPI(), jq.index, id)
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

type jobSorting types.SortOptions

func (s *jobSorting) SortCombinationsCaster() *types.SortCombinations {
	opts := &types.SortOptions{
		SortOptions: map[string]types.FieldSort{
			"job_next_run": {Order: &sortorder.Asc},
		},
	}
	c := types.SortCombinations(opts)
	return &c
}
