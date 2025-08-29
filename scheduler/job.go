// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	feeds "github.com/immanent-tech/go-syndication"
	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
)

var (
	_ quartz.ScheduledJob = (*ScheduledJob)(nil)
	_ quartz.Job          = (*UpdateFeedJob)(nil)
	_ quartz.Job          = (*GetNewFeedsJob)(nil)
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
	// defaultJobTimeout is the maximum duration a job can run before timing out and being cancelled.
	defaultJobTimeout = 60 * time.Second
	// feedJobGroup is a scheduler group key for jobs to fetch new items for feeds.
	feedJobGroup = "get_items"
)

// JobDetail defines additional job properties.
func (sj *ScheduledJob) JobDetail() *quartz.JobDetail {
	switch sj.JobType {
	case ScheduledJobJobTypeUpdateFeed:
		job, err := sj.JobData.AsUpdateFeedJob()
		if err == nil {
			if sj.JobOptions != nil {
				return quartz.NewJobDetailWithOptions(&job, GenerateJobKey(job.FeedID), sj.JobOptions)
			}
			return quartz.NewJobDetail(&job, GenerateJobKey(job.FeedID))
		}
	case ScheduledJobJobTypeGetNewFeeds:
		job, err := sj.JobData.AsGetNewFeedsJob()
		if err == nil {
			if sj.JobOptions != nil {
				return quartz.NewJobDetailWithOptions(&job, GenerateJobKey("get_new_feeds"), sj.JobOptions)
			}
			return quartz.NewJobDetail(&job, GenerateJobKey("get_new_feeds"))
		}
	}
	return nil
}

// Trigger defines the job trigger.
func (sj *ScheduledJob) Trigger() quartz.Trigger {
	switch sj.JobTriggerType {
	case ScheduledJobJobTriggerTypeCron:
		data, err := sj.JobTrigger.AsCronTrigger()
		if err != nil {
			trigger, _ := quartz.NewCronTrigger(defaultJobTrigger)
			return trigger
		}
		trigger, _ := quartz.NewCronTrigger(data.Schedule)
		return trigger
	case ScheduledJobJobTriggerTypePoll:
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

// MarshalJob takes a quartz.ScheduledJob object and marshals it back into a ScheduledJob, updating fields as
// appropriate.
func MarshalJob(job quartz.ScheduledJob) (*ScheduledJob, error) {
	serialized := &ScheduledJob{
		JobNextRun: time.Unix(0, job.NextRunTime()),
		CreatedAt:  time.Now().UTC(),
		JobOptions: job.JobDetail().Options(),
	}
	// Parse and generate trigger.
	switch trigger := ParseTrigger(job.Trigger()).(type) {
	case *PollTrigger:
		err := serialized.JobTrigger.FromPollTrigger(*trigger)
		if err != nil {
			return nil, errors.Join(ErrMarshalJobFailed, err)
		}
		serialized.JobTriggerType = ScheduledJobJobTriggerTypePoll
	case *CronTrigger:
		err := serialized.JobTrigger.FromCronTrigger(*trigger)
		if err != nil {
			return nil, errors.Join(ErrMarshalJobFailed, err)
		}
		serialized.JobTriggerType = ScheduledJobJobTriggerTypeCron
	}

	switch job := job.JobDetail().Job().(type) {
	case *UpdateFeedJob:
		err := serialized.JobData.FromUpdateFeedJob(*job)
		if err != nil {
			return nil, errors.Join(ErrMarshalJobFailed, err)
		}
		serialized.JobType = ScheduledJobJobTypeUpdateFeed
	case *GetNewFeedsJob:
		err := serialized.JobData.FromGetNewFeedsJob(*job)
		if err != nil {
			return nil, errors.Join(ErrMarshalJobFailed, err)
		}
		serialized.JobType = ScheduledJobJobTypeGetNewFeeds
	default:
		return nil, errors.Join(ErrMarshalJobFailed, ErrInvalidJob)
	}

	return serialized, nil
}

// NewUpdateFeedJob creates a job that can be scheduled from the given feed data.
func NewUpdateFeedJob(id models.FeedID, urls []models.URL, trigger *PollTrigger) (*ScheduledJob, error) {
	jobTrigger := ScheduledJob_JobTrigger{}
	err := jobTrigger.FromPollTrigger(*trigger)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateJobFailed, err)
	}
	job := &ScheduledJob{
		CreatedAt:      time.Now().UTC(),
		JobTrigger:     jobTrigger,
		JobTriggerType: ScheduledJobJobTriggerTypePoll,
		JobType:        ScheduledJobJobTypeUpdateFeed,
	}
	err = job.JobData.FromUpdateFeedJob(UpdateFeedJob{
		FeedID: id,
		URLs:   urls,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateJobFailed, err)
	}

	return job, nil
}

// Execute is called by the scheduler when the job is scheduled to run.
//
//nolint:nestif
func (job *UpdateFeedJob) Execute(ctx context.Context) error {
	jobCtx, cancel := context.WithTimeout(ctx, defaultJobTimeout)
	defer cancel()
	// Retrieve the feed details.
	details, err := manager.db.GetFeed(jobCtx, job.FeedID)
	if err != nil && !errors.Is(err, ErrNoJob) {
		return fmt.Errorf("job %s failed: %w", job.Description(), err)
	}
	// Get new items since the last fetch.
	feed, err := feeds.NewFeedFromURL(jobCtx, job.URLs[0])
	if err != nil {
		return fmt.Errorf("job %s failed: %w", job.Description(), err)
	}
	slogctx.FromCtx(ctx).Debug("Checking for new items.",
		slog.String("feed", details.GetTitle()),
		slog.Time("since", details.LastFetched),
	)
	items := make(models.Items, 0, len(feed.GetItems()))
	for i := range slices.Values(feed.GetItems()) {
		items = append(items, models.NewItemFromSource(&i, job.FeedID, string(feed.SourceType)))
	}
	// Add any new items since the last feed update.
	if len(items.FilterSince(details.LastFetched)) > 0 {
		// Add any new items.
		results, err := manager.db.AddItems(jobCtx, items.FilterSince(details.LastFetched)...)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrExecuteJobFailed, err)
		} else {
			for _, result := range results {
				if !result.Created() {
					slogctx.FromCtx(jobCtx).WarnContext(jobCtx, "Failing to index an item.",
						slog.String("feed", details.GetTitle()),
						slog.Any("error", result),
					)
				}
			}
		}
		// Update the feed details.
		err = manager.db.UpdateFeed(jobCtx, job.FeedID, nil)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrExecuteJobFailed, err)
		} else {
			slogctx.FromCtx(ctx).Debug("Added new items.",
				slog.String("feed", details.GetTitle()),
				slog.Int("count", len(items.FilterSince(details.LastFetched))),
			)
		}
	}
	return nil
}

// Description returns the description of the job.
func (job *UpdateFeedJob) Description() string {
	return "Update feed " + job.FeedID
}

// NewGetNewFeedsJob creates a job for checking for new feeds.
func NewGetNewFeedsJob() (*ScheduledJob, error) {
	jobTrigger := ScheduledJob_JobTrigger{}
	err := jobTrigger.FromPollTrigger(*NewPollTrigger(time.Minute, 5*time.Second))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateJobFailed, err)
	}
	job := &ScheduledJob{
		CreatedAt:      time.Now().UTC(),
		JobTrigger:     jobTrigger,
		JobTriggerType: ScheduledJobJobTriggerTypePoll,
		JobType:        ScheduledJobJobTypeGetNewFeeds,
	}
	err = job.JobData.FromGetNewFeedsJob(GetNewFeedsJob{
		Interval: time.Minute.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateJobFailed, err)
	}

	return job, nil
}

// Execute is called by the scheduler when the job is scheduled to run.
func (job *GetNewFeedsJob) Execute(ctx context.Context) error {
	state := &GetNewFeedsJobState{}
	lastState, err := manager.db.GetJobState(ctx, "get_new_feeds")
	if err != nil {
		if !errors.Is(err, elastic.ErrNotFound) {
			return fmt.Errorf("%w: %w", ErrScheduler, err)
		}
		state.Checkpoint = time.Time{}
	} else {
		err = json.Unmarshal(lastState.JobData, state)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrScheduler, err)
		}
	}
	slogctx.FromCtx(ctx).DebugContext(ctx, "Looking for new feeds.",
		slog.Time("since", state.Checkpoint),
	)
	feeds, err := manager.db.GetNewFeedsSince(ctx, state.Checkpoint)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrScheduler, err)
	}
	if len(feeds) > 0 {
		slogctx.FromCtx(ctx).DebugContext(ctx, "Found new feeds.",
			slog.Int("count", len(feeds)),
		)
	}
	// Update the checkpoint.
	state.Checkpoint = time.Now().UTC()
	err = manager.db.UpdateJobState(ctx, "get_new_feeds", map[string]any{
		"job_data": state,
	})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrScheduler, err)
	}

	// Create new feed jobs where necessary.
	for feed := range slices.Values(feeds) {
		var (
			job quartz.ScheduledJob
			err error
		)
		job, err = NewUpdateFeedJob(feed.GetID(), feed.SourceURLs, NewPollTrigger(defaultPollInterval, defaultPollJitter))
		if err != nil {
			slogctx.FromCtx(ctx).WarnContext(ctx, "Failed to schedule job for feed.",
				slog.String("feed_id", feed.GetID()),
				slog.Any("error", err))

			continue
		}
		// Check for existing job and schedule new job if needed.
		_, err = manager.scheduler.GetScheduledJob(job.JobDetail().JobKey())
		if err != nil && errors.Is(err, quartz.ErrJobNotFound) {
			err = manager.scheduler.ScheduleJob(job.JobDetail(), job.Trigger())
			if err != nil {
				slog.ErrorContext(ctx, "Failed to schedule new job for feed.",
					slog.Group("feed",
						slog.String("id", feed.GetID()),
						slog.String("title", feed.GetTitle()),
					),
					slog.Group("job",
						slog.String("id", job.JobDetail().JobKey().String()),
						slog.String("schedule", job.Trigger().Description()),
					),
					slog.Any("error", err),
				)
				continue
			}
			slogctx.FromCtx(ctx).DebugContext(ctx, "Added job for feed.",
				slog.Group("feed",
					slog.String("id", feed.GetID()),
					slog.String("title", feed.GetTitle()),
				),
				slog.Group("job",
					slog.String("id", job.JobDetail().JobKey().String()),
					slog.String("schedule", job.Trigger().Description()),
				),
			)
			// Do an initial run of the job.
			go func() {
				err = job.JobDetail().Job().Execute(ctx)
				if err != nil {
					slog.ErrorContext(ctx, "Failed initial run of update feed job.",
						slog.Group("feed",
							slog.String("id", feed.GetID()),
							slog.String("title", feed.GetTitle()),
						),
						slog.Group("job",
							slog.String("id", job.JobDetail().JobKey().String()),
							slog.String("schedule", job.Trigger().Description()),
						),
						slog.Any("error", err),
					)
				}
			}()
		}
	}
	return nil
}

// Description returns the description of the job.
func (job *GetNewFeedsJob) Description() string {
	return "Get new feeds"
}

// GenerateJobKey generates an appropriate job key based on the type of job.
func GenerateJobKey(jobID string) *quartz.JobKey {
	switch models.IdentifyID(jobID) {
	case models.FeedPFX:
		return quartz.NewJobKeyWithGroup(jobID, feedJobGroup)
	default:
		return quartz.NewJobKey(jobID)
	}
}
