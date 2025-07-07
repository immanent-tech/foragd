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

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/sortorder"
	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/logging"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/providers/elastic/schema"
)

// Make sure out jobQueue implementation satisfies quartz.JobQueue.
var _ quartz.JobQueue = (*JobQueue)(nil)

var schedCtx context.Context

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
)

// JobQueue implements the quartz.JobQueue interface, using Elasticsearch as the
// persistence layer.
type JobQueue struct {
	client *elastic.API
	logger *slog.Logger
	index  string
}

// NewJobQueue initializes and returns an empty jobQueue.
func NewJobQueue(ctx context.Context, client *elastic.API) (*JobQueue, error) {
	schedCtx = ctx

	return &JobQueue{
		client: client,
		logger: slogctx.FromCtx(ctx).WithGroup("job_queue"),
		index:  schema.SchedulerSchemaPrefix,
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

	found, err := elastic.Exists(schedCtx, jq.client.GetAPI(), jq.index, id)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPushJobFailed, err)
	}
	if found {
		if err := jq.delete(id); err != nil {
			return fmt.Errorf("%w: %w", ErrPushJobFailed, err)
		}
	}

	if err := elastic.CreateDoc(schedCtx, jq.client.GetAPI(), jq.index, id, data); err != nil {
		return fmt.Errorf("%w: %w", ErrPushJobFailed, err)
	}

	slogctx.FromCtx(schedCtx).Log(schedCtx, logging.LevelTrace, "Pushed job to queue.",
		slog.Group("job",
			slog.String("id", job.JobDetail().JobKey().String()),
		),
	)

	return nil
}

// Pop removes and returns the next scheduled job from the queue.
func (jq *JobQueue) Pop() (quartz.ScheduledJob, error) {
	job, err := jq.findHead()
	if err != nil {
		return nil, errors.Join(ErrPopJobFailed, err)
	}
	id := jobKeyToDocID(job.JobDetail().JobKey().String())

	if err := jq.delete(id); err != nil {
		return nil, errors.Join(ErrPopJobFailed, err)
	}

	slogctx.FromCtx(schedCtx).Log(schedCtx, logging.LevelTrace, "Popped job from queue.",
		slog.Group("job",
			slog.String("id", job.JobDetail().JobKey().String()),
		),
	)

	return job, nil
}

// Head returns the first scheduled job without removing it from the queue.
func (jq *JobQueue) Head() (quartz.ScheduledJob, error) {
	job, err := jq.findHead()
	if err != nil {
		jq.logger.Error("Failed to find first scheduled job.",
			slog.Any("error", err))
	}

	slog.Debug("Found next job.",
		slog.Group("job",
			slog.String("id", job.JobDetail().JobKey().String()),
			slog.Time("next_run", time.Unix(0, job.NextRunTime())),
		),
	)

	return job, err
}

// Get returns the scheduled job with the specified key without removing it
// from the queue.
func (jq *JobQueue) Get(jobKey *quartz.JobKey) (quartz.ScheduledJob, error) {
	id := jobKeyToDocID(jobKey.String())
	job, err := elastic.GetDoc[string, ScheduledJob](schedCtx, jq.client.GetAPI(), jq.index, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrGetJobFailed, err)
	}

	return &job, nil
}

// Remove removes and returns the scheduled job with the specified key.
func (jq *JobQueue) Remove(jobKey *quartz.JobKey) (quartz.ScheduledJob, error) {
	job, err := jq.Get(jobKey)
	if err != nil {
		return nil, errors.Join(ErrRemoveJobFailed, err)
	}
	id := jobKeyToDocID(jobKey.String())

	if err := jq.delete(id); err != nil {
		return nil, errors.Join(ErrRemoveJobFailed, err)
	}

	jq.logger.Debug("Job removed.",
		slog.String("job", job.JobDetail().Job().Description()))

	return job, nil
}

// ScheduledJobs returns the slice of all scheduled jobs in the queue.
func (jq *JobQueue) ScheduledJobs(matchers []quartz.Matcher[quartz.ScheduledJob]) ([]quartz.ScheduledJob, error) {
	searchSize := 1000
	pagination := make([]types.FieldValue, 0)
	allJobs := make([]ScheduledJob, 0)
	jobs := make([]quartz.ScheduledJob, 0)

	// Loop until we've paginated through all results.
	for {
		var (
			jobs []ScheduledJob
			err  error
		)

		jobs, pagination, err = elastic.Search[ScheduledJob](schedCtx, jq.client.GetAPI(), jq.index, query.MatchAll(), searchSize, nil, pagination)
		if err != nil {
			return nil, errors.Join(elastic.ErrSearchFailed, err)
		}

		allJobs = append(allJobs, jobs...)
		// Stop if the number of hits is less than the search size (i.e., last set of hits).
		if len(jobs) < searchSize {
			break
		}
	}

	for _, job := range allJobs {
		if isMatch(&job, matchers) {
			jobs = append(jobs, &job)
		}
	}

	return jobs, nil
}

// Size returns the size of the job queue.
func (jq *JobQueue) Size() (int, error) {
	count, err := elastic.Count(schedCtx, jq.client.GetAPI(), jq.index, query.MatchAll())
	if err != nil {
		return 0, fmt.Errorf("could not get size of scheduled jobs queue: %w", err)
	}
	return int(count), nil
}

// Clear clears the job queue.
func (jq *JobQueue) Clear() error {
	jq.logger.Debug("Cleared job queue.")
	return nil
}

func (jq *JobQueue) findHead() (quartz.ScheduledJob, error) {
	sort := []types.SortCombinations{
		types.SortOptions{
			SortOptions: map[string]types.FieldSort{
				"job_next_run": {Order: &sortorder.Asc},
			},
		},
	}
	jobs, _, err := elastic.Search[*ScheduledJob](schedCtx, jq.client.GetAPI(), jq.index, query.MatchAll(), 1, sort, nil)
	if err != nil {
		return nil, errors.Join(ErrNoJobFound, err)
	}
	if len(jobs) == 0 {
		return nil, errors.Join(ErrNoJobFound, err)
	}

	nextJob := jobs[0]
	return nextJob, nil
}

// delete removes the job doc from Elasticsearch.
func (jq *JobQueue) delete(id string) error {
	err := elastic.DeleteDoc(schedCtx, jq.client.GetAPI(), jq.index, id)
	if err != nil {
		return errors.Join(ErrDeleteJobFailed, err)
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
