// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
)

var (
	_ quartz.ScheduledJob = (*ScheduledJob)(nil)
	_ quartz.Job          = (*FeedJob)(nil)
)

var (
	ErrExecuteJobFailed = errors.New("could not execute job")
	ErrCreateJobFailed  = errors.New("create job failed")
	ErrMarshalJobFailed = errors.New("could not marshal job")
	ErrInvalidJob       = errors.New("invalid job")
	ErrNoJob            = errors.New("no job found")
)

const (
	// defaultJobTrigger is a cron schedule to run a job every 5 minutes.
	defaultJobTrigger = "0 */5 * * * *"
	// FeedJobGroup is a scheduler group key for jobs to fetch new items for
	// feeds.
	FeedJobGroup = "get_items"
)

// JobDetail defines additional job properties.
func (sj *ScheduledJob) JobDetail() *quartz.JobDetail {
	job, err := sj.JobData.AsFeedJob()
	if err != nil {
		return nil
	}

	if sj.JobOptions != nil {
		return quartz.NewJobDetailWithOptions(&job, GenerateJobKey(job.FeedID), sj.JobOptions)
	}

	return quartz.NewJobDetail(&job, GenerateJobKey(job.FeedID))
}

// Trigger defines the job trigger.
func (sj *ScheduledJob) Trigger() quartz.Trigger {
	switch sj.JobType {
	case Cron:
		data, err := sj.JobTrigger.AsCronTrigger()
		if err != nil {
			trigger, _ := quartz.NewCronTrigger(defaultJobTrigger)
			return trigger
		}
		trigger, _ := quartz.NewCronTrigger(data.Schedule)
		return trigger
	case Poll:
		trigger, err := sj.JobTrigger.AsPollTrigger()
		if err != nil {
			return NewPollTrigger(defaultPollInterval, defaultPollJitter)
		}
		return &trigger
	}
	return NewPollTrigger(defaultPollInterval, defaultPollJitter)
}

// NextRunTime returns the next scheduled run time for the job.
func (sj *ScheduledJob) NextRunTime() int64 {
	return sj.JobNextRun.UnixNano()
}

// MarshalJob takes a quartz.ScheduledJob object and marshals it back into a
// ScheduledJob, updating fields as appropriate.
func MarshalJob(job quartz.ScheduledJob) (*ScheduledJob, error) {
	serialized := &ScheduledJob{
		JobNextRun: time.Unix(0, job.NextRunTime()),
		CreatedAt:  time.Now().UTC(),
		JobOptions: job.JobDetail().Options(),
	}
	// Parse and generate trigger.
	switch trigger := ParseTrigger(job.Trigger()).(type) {
	case *PollTrigger:
		if err := serialized.JobTrigger.FromPollTrigger(*trigger); err != nil {
			return nil, errors.Join(ErrMarshalJobFailed, err)
		}
		serialized.JobType = Poll
	case *CronTrigger:
		if err := serialized.JobTrigger.FromCronTrigger(*trigger); err != nil {
			return nil, errors.Join(ErrMarshalJobFailed, err)
		}
		serialized.JobType = Cron
	}

	switch job := job.JobDetail().Job().(type) {
	case *FeedJob:
		if err := serialized.JobData.FromFeedJob(*job); err != nil {
			return nil, errors.Join(ErrMarshalJobFailed, err)
		}
	default:
		return nil, errors.Join(ErrMarshalJobFailed, ErrInvalidJob)
	}

	return serialized, nil
}

// Execute is called by the scheduler when the job is scheduled to run.
func (job *FeedJob) Execute(ctx context.Context) error {
	api := FeedManagementAPIFromCtx(ctx)
	if api == nil {
		return fmt.Errorf("%w: no feed management api in context", ErrExecuteJobFailed)
	}

	// Retrieve the feed details.
	feeds, err := api.GetFeeds(ctx, job.FeedID)
	if err != nil && !errors.Is(err, ErrNoJob) {
		return fmt.Errorf("%w: %w", ErrExecuteJobFailed, err)
	}

	// Get new items since the last fetch.
	items, err := job.getItemsSince(feeds[0].Updated)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrExecuteJobFailed, err)
	}
	if len(items) > 0 {
		// Add any new items.
		if resp, err := api.AddItems(ctx, items...); err != nil || resp.Err != nil {
			return fmt.Errorf("%w: %w", ErrExecuteJobFailed, err)
		}
		// Update the feed timestamp.
		if err := api.MarkFeedUpdated(ctx, job.FeedID); err != nil {
			return fmt.Errorf("%w: %w", ErrExecuteJobFailed, err)
		}
		slogctx.FromCtx(ctx).Debug("Job execution finished.",
			slog.String("job", job.Description()),
			slog.Time("updated_at", feeds[0].Updated),
			slog.Int("items_added", len(items)),
		)
	}

	return nil
}

// Description returns the description of the job.
func (job *FeedJob) Description() string {
	return fmt.Sprintf("Update feed %s (%s)", job.FeedID, job.URL)
}

// getItemsSince retrieves the feed items that are newer than the given time.
func (job *FeedJob) getItemsSince(since time.Time) ([]*models.Item, error) {
	items, err := models.GetFeedItems(schedCtx, job.FeedID, job.URL)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrExecuteJobFailed, err)
	}

	return items.FilterSince(since), nil
}

// NewFeedJob creates a job that can be scheduled from the given feed data.
func NewFeedJob(id models.FeedID, url models.URL, trigger *PollTrigger) (*ScheduledJob, error) {
	jobTrigger := ScheduledJob_JobTrigger{}
	err := jobTrigger.FromPollTrigger(*trigger)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateJobFailed, err)
	}

	job := &ScheduledJob{
		CreatedAt:  time.Now().UTC(),
		JobTrigger: jobTrigger,
	}

	if err := job.JobData.FromFeedJob(FeedJob{
		FeedID: id,
		URL:    url,
	}); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateJobFailed, err)
	}

	return job, nil
}

// GenerateJobKey generates an appropriate job key based on the type of job.
//
//nolint:gocritic // more cases to be implemented
func GenerateJobKey(jobID string) *quartz.JobKey {
	switch models.IdentifyID(jobID) {
	case models.FeedPFX:
		return quartz.NewJobKeyWithGroup(jobID, FeedJobGroup)
	}

	return nil
}
