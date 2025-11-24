// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"slices"
	"strings"
	"time"

	feeds "github.com/immanent-tech/go-syndication"
	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"
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
	defaultCronJobTrigger = "0 */5 * * * *"
	defaultPollInterval   = time.Minute
	defaultPollJitter     = 5 * time.Second
	defaultJobTimeout     = 60 * time.Second
	pollTriggerID         = "PollTrigger"
	cronTriggerID         = "CronTrigger"
)

type SchedulerAPI interface {
	GetJobState(ctx context.Context, id string) (*JobState, error)
	UpdateJobState(ctx context.Context, id string, updates map[string]any) error
	ScheduleJob(jobDetail *quartz.JobDetail, trigger quartz.Trigger) error
	GetScheduledJob(jobKey *quartz.JobKey) (quartz.ScheduledJob, error)
}

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
			trigger, _ := quartz.NewCronTrigger(defaultCronJobTrigger)
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
	switch trigger := parseTrigger(job.Trigger()).(type) {
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
func NewUpdateFeedJob(id FeedID, urls []URL, trigger *PollTrigger) (*ScheduledJob, error) {
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
func (job *UpdateFeedJob) Execute(ctx context.Context) error {
	api := DataAPIFromCtx(ctx)
	if api == nil {
		return fmt.Errorf("%w: execute update feed job: no api in context", ErrExecuteJobFailed)
	}

	// Add feed id as slog attribute for log tracking.
	ctx = slogctx.With(ctx, "feed_id", job.FeedID)

	jobCtx, cancel := context.WithTimeout(ctx, defaultJobTimeout)
	defer cancel()

	// Retrieve the feed details.
	details, err := api.GetFeed(jobCtx, job.FeedID)
	if err != nil && !errors.Is(err, ErrNoJob) {
		return fmt.Errorf("%w: %s: %w", ErrExecuteJobFailed, job.Description(), err)
	}
	// Get new items since the last fetch.
	feed, err := feeds.NewFeedFromURL(jobCtx, job.URLs[0])
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrExecuteJobFailed, job.Description(), err)
	}
	slogctx.FromCtx(ctx).Debug("Checking for new items.",
		slog.String("feed", details.GetTitle()),
		slog.Time("since", details.LastFetched),
	)
	items := make(Items, 0)
	for i := range slices.Values(feed.GetItems()) {
		items = append(items, NewItemFromSource(&i, details))
	}
	// Add any new items since the last feed update.
	if len(items.FilterSince(details.LastFetched)) > 0 {
		// Add any new items.
		_, err = api.AddItems(jobCtx, items.FilterSince(details.LastFetched)...)
		if err != nil {
			return fmt.Errorf("%w: %s: %w", ErrExecuteJobFailed, job.Description(), err)
		}
		// Update the feed details.
		err = api.UpdateFeed(jobCtx, job.FeedID, nil)
		if err != nil {
			return fmt.Errorf("%w: %s: %w", ErrExecuteJobFailed, job.Description(), err)
		}
		slogctx.FromCtx(ctx).Debug("Added new items.",
			slog.String("feed", details.GetTitle()),
			slog.Int("count", len(items.FilterSince(details.LastFetched))),
		)
	}
	return nil
}

// Description returns the description of the job.
func (job *UpdateFeedJob) Description() string {
	return "Update feed checks for new items in the feed " + job.FeedID
}

// NewGetNewFeedsJob creates a job for checking for new feeds.
func NewGetNewFeedsJob() (*ScheduledJob, error) {
	jobTrigger := ScheduledJob_JobTrigger{}
	err := jobTrigger.FromPollTrigger(*NewPollTrigger(defaultPollInterval, defaultPollJitter))
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
//
//nolint:gocognit,funlen
func (job *GetNewFeedsJob) Execute(ctx context.Context) error {
	schedulerAPI := SchedulerAPIFromCtx(ctx)
	if schedulerAPI == nil {
		return fmt.Errorf("%w: %s: no scheduler api in context", ErrExecuteJobFailed, job.Description())
	}

	dataAPI := DataAPIFromCtx(ctx)
	if dataAPI == nil {
		return fmt.Errorf("%w: %s: no data api in context", ErrExecuteJobFailed, job.Description())
	}

	state := &GetNewFeedsJobState{}
	lastState, err := schedulerAPI.GetJobState(ctx, "get_new_feeds")
	if err != nil {
		if HTTPStatus(err) != http.StatusNotFound {
			return fmt.Errorf("%w: %s: %w", ErrExecuteJobFailed, job.Description(), err)
		}
		state.Checkpoint = time.Time{}
	} else {
		err = json.Unmarshal(lastState.JobData, state)
		if err != nil {
			return fmt.Errorf("%w: %s: %w", ErrExecuteJobFailed, job.Description(), err)
		}
	}
	slogctx.FromCtx(ctx).DebugContext(ctx, "Looking for new feeds.",
		slog.Time("since", state.Checkpoint),
	)
	feeds, err := dataAPI.GetNewFeedsSince(ctx, state.Checkpoint)
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrExecuteJobFailed, job.Description(), err)
	}
	if len(feeds) > 0 {
		slogctx.FromCtx(ctx).DebugContext(ctx, "Found new feeds.",
			slog.Int("count", len(feeds)),
		)
	}
	// Update the checkpoint.
	state.Checkpoint = time.Now().UTC()
	err = schedulerAPI.UpdateJobState(ctx, "get_new_feeds_state", map[string]any{
		"job_data": state,
	})
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrExecuteJobFailed, job.Description(), err)
	}

	// Create new feed jobs where necessary.
	for feed := range slices.Values(feeds) {
		var job quartz.ScheduledJob
		job, err = NewUpdateFeedJob(
			feed.GetID(),
			feed.SourceURLs,
			NewPollTrigger(defaultPollInterval, defaultPollJitter),
		)
		if err != nil {
			slogctx.FromCtx(ctx).WarnContext(ctx, "Failed to schedule job for feed.",
				slog.String("feed_id", feed.GetID()),
				slog.Any("error", err))
			continue
		}
		// Check for existing job and schedule new job if needed.
		_, err = schedulerAPI.GetScheduledJob(job.JobDetail().JobKey())
		if err != nil && errors.Is(err, quartz.ErrJobNotFound) {
			err = schedulerAPI.ScheduleJob(job.JobDetail(), job.Trigger())
			if err != nil {
				slogctx.FromCtx(ctx).Error("Failed to schedule new job for feed.",
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
					slogctx.FromCtx(ctx).Error("Failed initial run of update feed job.",
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
	return "Get new feeds checks for any new feeds added since the last job run."
}

// GenerateJobKey generates an appropriate job key based on the type of job.
func GenerateJobKey(jobID string) *quartz.JobKey {
	switch {
	case strings.HasPrefix(jobID, "feed_"):
		return quartz.NewJobKeyWithGroup(jobID, string(ScheduledJobJobTypeUpdateFeed))
	default:
		return quartz.NewJobKeyWithGroup(jobID, string(ScheduledJobJobTypeGetNewFeeds))
	}
}

// Verify PollTrigger satisfies the Trigger interface.
var _ quartz.Trigger = (*PollTrigger)(nil)

// NewPollTrigger returns a new polling job using the given interval and jitter.
func NewPollTrigger(interval, jitter any) *PollTrigger {
	return &PollTrigger{
		Interval: asDuration(interval, defaultPollInterval),
		Jitter:   asDuration(jitter, defaultPollJitter),
	}
}

// NextFireTime returns the next time at which the PollTriggerWithJitter is scheduled to fire.
func (t *PollTrigger) NextFireTime(prev int64) (int64, error) {
	jitter := rand.NormFloat64()*float64(t.Jitter) + float64(t.Interval) // #nosec: G404
	next := prev + int64(jitter)
	return next, nil
}

// Description returns the description of the PollTriggerWithJitter.
func (t *PollTrigger) Description() string {
	return strings.Join([]string{pollTriggerID, t.Interval.String(), t.Jitter.String()}, quartz.Sep)
}

// parseTrigger will attempt to parse the given trigger interface into its concrete trigger type. If the interface value
// cannot be parsed, a default polling trigger will be returned.
func parseTrigger(trigger quartz.Trigger) any {
	desc := trigger.Description()
	triggerOpts := strings.Split(desc, quartz.Sep)
	switch {
	case strings.HasPrefix(desc, pollTriggerID):
		if len(triggerOpts) != 3 { //nolint:mnd // this is a very specific check.
			return NewPollTrigger(defaultPollInterval, defaultPollJitter)
		}
		return NewPollTrigger(triggerOpts[1], triggerOpts[2])
	case strings.HasPrefix(desc, cronTriggerID):
		return &CronTrigger{Schedule: triggerOpts[1]}
	}
	return NewPollTrigger(defaultPollInterval, defaultPollJitter)
}

// asDuration will attempt to parse the given input value as a duration. If the value cannot be parsed, the given
// fallback will be returned instead.
func asDuration(input any, fallback time.Duration) time.Duration {
	switch value := input.(type) {
	case time.Duration:
		return value
	case string:
		dur, err := time.ParseDuration(value)
		if err != nil {
			return fallback
		}
		return dur
	}
	return fallback
}
