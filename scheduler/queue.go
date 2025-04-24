// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/refresh"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/sortorder"
	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

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

// Push inserts a new scheduled job to the queue.
// This method is also used by the Scheduler to reschedule existing jobs that
// have been dequeued for execution.
func (jq *JobQueue) Push(job quartz.ScheduledJob) error {
	data, err := MarshalJob(job)
	if err != nil {
		return errors.Join(ErrPushJobFailed, err)
	}

	// Add to Elasticsearch.
	_, err = elastic.NewDocCreateRequest(jq.client.GetAPI(),
		jq.index,
		job.JobDetail().JobKey().String(),
		data,
		refresh.True,
	).Do(schedCtx)
	if err != nil {
		return errors.Join(ErrPushJobFailed, err)
	}

	return nil
}

// Pop removes and returns the next scheduled job from the queue.
func (jq *JobQueue) Pop() (quartz.ScheduledJob, error) {
	job, err := jq.findHead()
	if err != nil {
		return nil, errors.Join(ErrPopJobFailed, err)
	}

	if err := jq.delete(job.JobDetail().JobKey().String()); err != nil {
		return nil, errors.Join(ErrPopJobFailed, err)
	}

	return job, nil
}

// Head returns the first scheduled job without removing it from the queue.
func (jq *JobQueue) Head() (quartz.ScheduledJob, error) {
	job, err := jq.findHead()
	if err != nil {
		jq.logger.Error("Failed to find first scheduled job.",
			slog.Any("error", err))
	}

	return job, err
}

// Get returns the scheduled job with the specified key without removing it
// from the queue.
func (jq *JobQueue) Get(jobKey *quartz.JobKey) (quartz.ScheduledJob, error) {
	resp, err := elastic.NewGetRequest(jq.client.GetAPI(),
		jq.index,
		jobKey.String(),
	).Do(schedCtx)
	if err != nil {
		return nil, errors.Join(ErrGetJobFailed, err)
	}

	job, err := elastic.ExtractSource[ScheduledJob](resp.Source_)
	if err != nil {
		return nil, errors.Join(ErrGetJobFailed, err)
	}

	return &job, nil
}

// Remove removes and returns the scheduled job with the specified key.
func (jq *JobQueue) Remove(jobKey *quartz.JobKey) (quartz.ScheduledJob, error) {
	job, err := jq.Get(jobKey)
	if err != nil {
		return nil, errors.Join(ErrRemoveJobFailed, err)
	}

	if err := jq.delete(jobKey.String()); err != nil {
		return nil, errors.Join(ErrRemoveJobFailed, err)
	}

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
			jobs     []ScheduledJob
			warnings error
		)

		resp, err := elastic.NewSearchRequest(jq.client.GetAPI(),
			elastic.WithSearchIndex(jq.index),
			elastic.WithSearchQueryOptions(query.MatchAll()),
			elastic.WithSearchSize(searchSize),
			elastic.WithSearchAfter(pagination),
		).Do(schedCtx)
		if err != nil {
			return nil, errors.Join(elastic.ErrSearchFailed, err)
		}
		// Stop if there are no hits
		if len(resp.Hits.Hits) == 0 {
			return nil, nil
		}
		// Loop through this set of results.
		jobs, pagination, warnings = elastic.ExtractSourceFromHits[ScheduledJob](resp.Hits.Hits)
		if warnings != nil {
			jq.logger.Warn("Could not extract all jobs.",
				slog.Any("warnings", warnings))
		}

		allJobs = append(allJobs, jobs...)
		// Stop if the number of hits is less than the search size (i.e., last set of hits).
		if len(resp.Hits.Hits) < searchSize {
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
	resp, err := elastic.NewCountRequest(jq.client.GetAPI(),
		elastic.WithCountIndex(jq.index),
		elastic.WithCountQueryOptions(
			query.MatchAll(),
		),
	).Do(schedCtx)
	if err != nil {
		return 0, fmt.Errorf("could not get size of scheduled jobs queue: %w", err)
	}

	jq.logger.Debug("Retrieved job queue size.",
		slog.Int64("size", resp.Count))

	return int(resp.Count), nil
}

// Clear clears the job queue.
func (jq *JobQueue) Clear() error {
	jq.logger.Debug("Cleared job queue.")
	return nil
}

func (jq *JobQueue) findHead() (quartz.ScheduledJob, error) {
	resp, err := elastic.NewSearchRequest(jq.client.GetAPI(),
		elastic.WithSearchIndex(jq.index),
		elastic.WithSearchQueryOptions(
			query.MatchAll(),
		),
		elastic.WithSearchSize(1),
		elastic.WithSortOptions(map[string]types.FieldSort{"job_next_run": {Order: &sortorder.Asc}}),
	).Do(schedCtx)
	if err != nil {
		return nil, errors.Join(ErrNoJobFound, err)
	}
	// Stop if there are no hits
	if len(resp.Hits.Hits) == 0 {
		return nil, errors.Join(ErrNoJobFound, err)
	}

	// Loop through this set of results.
	nextJob, err := elastic.ExtractSource[ScheduledJob](resp.Hits.Hits[0].Source_)
	if err != nil {
		return nil, errors.Join(ErrParseJobFailed, err)
	}

	// jq.logger.Debug("Found next job.", slog.Any("job", nextJob))

	return &nextJob, nil
}

// delete removes the job doc from Elasticsearch.
func (jq *JobQueue) delete(id string) error {
	_, err := elastic.NewDocDeleteRequest(jq.client.GetAPI(),
		jq.index,
		id,
		refresh.True,
	).Do(schedCtx)
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

// NewJobQueue initializes and returns an empty jobQueue.
func NewJobQueue(ctx context.Context, client *elastic.API) (*JobQueue, error) {
	schedCtx = ctx

	return &JobQueue{
		client: client,
		logger: slogctx.FromCtx(ctx).WithGroup("job_queue"),
		index:  schema.SchedulerJobsPrefix,
	}, nil
}
