// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/refresh"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/sortorder"
	"github.com/reugn/go-quartz/logger"
	"github.com/reugn/go-quartz/quartz"

	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/schema"
)

// Make sure out jobQueue implementation satisfies quartz.JobQueue.
var _ quartz.JobQueue = (*JobQueue)(nil)

var schedCtx context.Context

var (
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
	client *Client
	logger *slog.Logger
}

// NewJobQueue initializes and returns an empty jobQueue.
func NewJobQueue(ctx context.Context, client *Client, logger *slog.Logger) *JobQueue {
	schedCtx = ctx

	return &JobQueue{
		client: client,
		logger: logger,
	}
}

// Push inserts a new scheduled job to the queue.
// This method is also used by the Scheduler to reschedule existing jobs that
// have been dequeued for execution.
func (jq *JobQueue) Push(job quartz.ScheduledJob) error {
	jq.logger.Debug("Pushing job to queue.",
		slog.String("key", job.JobDetail().JobKey().String()),
	)

	jq.logger.Debug("Adding job.",
		slog.Any("job", job.JobDetail().Job()),
		slog.Int64("next_run", job.NextRunTime()),
	)

	data, err := models.MarshalJob(job)
	if err != nil {
		return errors.Join(ErrPushJobFailed, err)
	}

	// Add to Elasticsearch.
	_, err = jq.client.NewDocCreateRequest(
		schema.SchedulerJobsPrefix+"_test",
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
	logger.Trace("Pop")

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
	logger.Trace("Head")

	job, err := jq.findHead()
	if err != nil {
		logger.Errorf("Failed to find job: %s", err)
	}

	return job, err
}

func (jq *JobQueue) findHead() (quartz.ScheduledJob, error) {
	resp, err := jq.client.NewSearchRequest(
		WithIndexPattern[*search.Search](schema.SchedulerJobsPrefix+"_*"),
		WithSearchQueryOptions(
			QueryMatchAll(),
		),
		WithSearchSize(1),
		WithSortOptions(map[string]types.FieldSort{
			"job_next_run": {
				Order: &sortorder.Asc,
			},
		}),
	).Do(schedCtx)
	if err != nil {
		return nil, errors.Join(ErrNoJobFound, err)
	}
	// Stop if there are no hits
	if len(resp.Hits.Hits) == 0 {
		return nil, errors.Join(ErrNoJobFound, err)
	}

	// Loop through this set of results.
	nextJob, err := extractSource[models.ScheduledJob](resp.Hits.Hits[0].Source_)
	if err != nil {
		return nil, errors.Join(ErrParseJobFailed, err)
	}

	// jq.logger.Debug("Found next job.", slog.Any("job", nextJob))

	return &nextJob, nil
}

// Get returns the scheduled job with the specified key without removing it
// from the queue.
func (jq *JobQueue) Get(jobKey *quartz.JobKey) (quartz.ScheduledJob, error) {
	resp, err := jq.client.NewGetRequest(
		schema.SchedulerJobsPrefix+"_test",
		jobKey.String(),
	).Do(schedCtx)
	if err != nil {
		return nil, errors.Join(ErrGetJobFailed, err)
	}

	job, err := extractSource[models.ScheduledJob](resp.Source_)
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
	allJobs := make([]models.ScheduledJob, 0)
	jobs := make([]quartz.ScheduledJob, 0)

	// Loop until we've paginated through all results.
	for {
		resp, err := jq.client.NewSearchRequest(
			WithIndexPattern[*search.Search](schema.SchedulerJobsPrefix+"-*"),
			WithSearchQueryOptions(QueryMatchAll()),
			WithSearchSize(searchSize),
			WithSearchAfter(pagination),
		).Do(schedCtx)
		if err != nil {
			return nil, errors.Join(ErrSearchFailed, err)
		}
		// Stop if there are no hits
		if len(resp.Hits.Hits) == 0 {
			return nil, nil
		}
		// Loop through this set of results.
		allJobs = append(allJobs, extractSources[models.ScheduledJob](schedCtx, resp.Hits.Hits)...)
		// Update pagination value.
		pagination = resp.Hits.Hits[len(resp.Hits.Hits)-1].Sort
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

func isMatch(job quartz.ScheduledJob, matchers []quartz.Matcher[quartz.ScheduledJob]) bool {
	for _, matcher := range matchers {
		// require all matchers to match the job
		if !matcher.IsMatch(job) {
			return false
		}
	}

	return true
}

// Size returns the size of the job queue.
func (jq *JobQueue) Size() (int, error) {
	resp, err := jq.client.NewCountRequest(
		WithCountQueryOptions(
			QueryMatchAll(),
		),
	).Do(schedCtx)
	if err != nil {
		return 0, fmt.Errorf("could not get size of scheduled jobs queue: %w", err)
	}

	return int(resp.Count), nil
}

// Clear clears the job queue.
func (jq *JobQueue) Clear() error {
	return nil
}

// delete removes the job doc from Elasticsearch.
func (jq *JobQueue) delete(id string) error {
	_, err := jq.client.NewDocDeleteRequest(
		schema.SchedulerJobsPrefix+"_test",
		id,
		refresh.True,
	).Do(schedCtx)
	if err != nil {
		return errors.Join(ErrDeleteJobFailed, err)
	}

	return nil
}

func (c *Client) GetFeedJobState(ctx context.Context, feedID models.FeedID) (models.FeedJobState, error) {
	resp, err := c.NewGetRequest(schema.SchedulerStatePrefix+"_test", feedID).Do(ctx)
	if err != nil {
		return models.FeedJobState{ID: feedID, LastFetched: time.Time{}}, errors.Join(ErrGetJobState, err)
	}

	// Stop if there are no hits
	if !resp.Found {
		return models.FeedJobState{ID: feedID, LastFetched: time.Time{}}, errors.Join(ErrGetJobState, models.ErrNoJob)
	}

	// Loop through this set of results.
	state, err := extractSource[models.FeedJobState](resp.Source_)
	if err != nil {
		return models.FeedJobState{ID: feedID, LastFetched: time.Time{}}, errors.Join(ErrParseJobFailed, err)
	}

	return state, nil
}

// UpdateFeedJobState will update the job for a feed in the scheduler jobs
// index. Specifically, it will update the last_fetched value indicating when
// the feed last fetched its items.
func (c *Client) UpdateFeedJobState(ctx context.Context, feedID models.FeedID, lastFetched time.Time) error {
	if _, err := c.NewDocUpdateRequest(schema.SchedulerStatePrefix+"_test", feedID,
		WithDocUpdate(&models.FeedJobState{
			ID:          feedID,
			LastFetched: lastFetched,
		}, true),
	).Do(ctx); err != nil {
		return errors.Join(ErrUpdateJobStateFailed, err)
	}

	return nil
}

// FeedJobExists checks if a job for a feed already exists in the scheduler jobs index.
func (c *Client) FeedJobExists(ctx context.Context, feedID models.FeedID) (bool, error) {
	found, err := c.NewDocExistsRequest(schema.SchedulerJobsPrefix+"_test", feedID).Do(ctx)
	if err != nil {
		return false, errors.Join(ErrExistsFailed, err)
	}

	return found, nil
}
