// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package jobs implements a common type for quartz jobs and specific types and methods to execute different kinds of
// jobs.
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/reugn/go-quartz/quartz"

	"github.com/immanent-tech/foragd/models"
)

var (
	_ quartz.Job          = (*ScheduledJob)(nil)
	_ quartz.ScheduledJob = (*ScheduledJob)(nil)
)

var (
	ErrExecuteJobFailed = errors.New("could not execute job")
	ErrCreateJobFailed  = errors.New("create job failed")
	ErrMarshalJobFailed = errors.New("could not marshal job")
	ErrInvalidJob       = errors.New("invalid job")
	ErrNoJob            = errors.New("no job found")
)

const (
	defaultJobTimeout = 60 * time.Second

	jobTypeUpdateFeed           = "update_feed"
	jobTypeGetNewFeeds          = "get_new_feeds"
	jobTypeClearDeletedFeeds    = "clear_deleted_feeds"
	jobTypeClearExpiredSessions = "clear_expired_sessions"

	jobTriggerTypeCron = "cron"
	jobTriggerTypePoll = "poll"

	schedulerAPICtxKey contextKey = "scheduler_api"

	// defaultPaginationSize is the default number of docs to fetch when paginating through results from elasticsearch.
	defaultPaginationSize = 5000
)

type contextKey string

type SchedulerAPI interface {
	GetScheduledJob(jobKey *quartz.JobKey) (quartz.ScheduledJob, error)
	ScheduleJob(jobDetail *quartz.JobDetail, trigger quartz.Trigger) error
	DeleteJob(jobKey *quartz.JobKey) error
	GetJobState(ctx context.Context, id string) (*models.JobState, error)
	UpdateJobState(ctx context.Context, id string, updates map[string]any) error
}

func SchedulerAPIToCtx(ctx context.Context, schedulerAPI SchedulerAPI) context.Context {
	return context.WithValue(ctx, schedulerAPICtxKey, schedulerAPI)
}

// cronTrigger represents a trigger that runs on a Cron schedule.
type cronTrigger struct {
	Schedule string `json:"schedule" validate:"required,cron"`
}

// pollTrigger represents a polling trigger for a job.
type pollTrigger struct {
	Interval time.Duration `json:"interval" validate:"required"`
	Jitter   time.Duration `json:"jitter"   validate:"required,len"`
}

// ScheduledJob represents a job that has been scheduled by the job scheduler.
type ScheduledJob struct {
	// CreatedAt records when the object was created in the database.
	CreatedAt models.CreatedAt `json:"created_at"                     validate:"required"`
	// JobData contains job-specific data.
	JobData json.RawMessage `json:"job_data"`
	// JobNextRun is the next run time of the job.
	JobNextRun time.Time `json:"job_next_run"`
	// JobOptions are additional options for the job
	JobOptions *quartz.JobDetailOptions `json:"job_options,omitempty,omitzero"`
	// JobTrigger is the trigger for the job.
	JobTrigger json.RawMessage `json:"job_trigger"                    validate:"required"`
	// JobTriggerType is the type of trigger the job is using.
	JobTriggerType string `json:"job_trigger_type"               validate:"oneof=cron poll"`
	// JobDescription is a summary of what the job does.
	JobDescription string `json:"job_description"`
	// JobType is the type of job.
	JobType string `json:"job_type"                       validate:"oneof=update_feed get_new_feeds"`
}

// Description returns the description of the Job.
func (job *ScheduledJob) Description() string {
	return job.JobDescription
}

// Execute is called by a Scheduler when the Trigger associated with this job fires.
func (job *ScheduledJob) Execute(ctx context.Context) error {
	var err error
	switch job.JobType {
	case jobTypeGetNewFeeds:
		err = executeGetNewFeedsJob(ctx, job)
	case jobTypeUpdateFeed:
		err = executeUpdateFeedJob(ctx, job)
	case jobTypeClearDeletedFeeds:
		err = executeClearDeletedFeeds(ctx, job)
	case jobTypeClearExpiredSessions:
		err = executeClearExpiredSessions(ctx)
	}
	if err != nil {
		return fmt.Errorf("%w: %w", ErrExecuteJobFailed, err)
	}
	return nil
}

// JobDetail returns a quartz.JobDetail object for the job.
func (job *ScheduledJob) JobDetail() *quartz.JobDetail {
	switch job.JobType {
	case jobTypeGetNewFeeds:
		var data GetNewFeedsJobData
		if err := json.Unmarshal(job.JobData, &data); err != nil {
			return nil
		}
		if job.JobOptions != nil {
			return quartz.NewJobDetailWithOptions(job, job.generateJobKey("get_new_feeds", ""), job.JobOptions)
		}
		return quartz.NewJobDetail(job, job.generateJobKey("get_new_feeds", ""))
	case jobTypeUpdateFeed:
		var data UpdateFeedJobData
		if err := json.Unmarshal(job.JobData, &data); err != nil {
			return nil
		}
		if job.JobOptions != nil {
			return quartz.NewJobDetailWithOptions(job, job.generateJobKey(data.FeedID, job.JobType), job.JobOptions)
		}
		return quartz.NewJobDetail(job, job.generateJobKey(data.FeedID, job.JobType))
	case jobTypeClearDeletedFeeds:
		return quartz.NewJobDetail(job, job.generateJobKey("clear_delete_feeds", ""))
	case jobTypeClearExpiredSessions:
		return quartz.NewJobDetail(job, job.generateJobKey("clear_expired_sessions", ""))
	}
	return nil
}

// Trigger defines the job trigger.
func (job *ScheduledJob) Trigger() quartz.Trigger {
	switch job.JobTriggerType {
	case jobTriggerTypeCron:
		var body cronTrigger
		if err := json.Unmarshal(job.JobTrigger, &body); err != nil {
			trigger, _ := quartz.NewCronTrigger(defaultCronJobTrigger)
			return trigger
		}
		trigger, _ := quartz.NewCronTrigger(body.Schedule)
		return trigger
	case jobTriggerTypePoll:
		var body pollTrigger
		if err := json.Unmarshal(job.JobTrigger, &body); err != nil {
			return newPollTrigger(defaultPollInterval, defaultPollJitter)
		}
		return newPollTrigger(body.Interval, body.Jitter)
	}
	return newPollTrigger(defaultPollInterval, defaultPollJitter)
}

// NextRunTime returns the next scheduled run time for the job.
func (job *ScheduledJob) NextRunTime() int64 {
	return job.JobNextRun.UnixNano()
}

// generateJobKey generates an appropriate job key based on the type of job.
func (job *ScheduledJob) generateJobKey(jobID, group string) *quartz.JobKey {
	if group != "" {
		return quartz.NewJobKeyWithGroup(jobID, job.JobType)
	}
	return quartz.NewJobKey(jobID)
}

// MarshalJob takes a quartz.ScheduledJob object and marshals it back into a ScheduledJob, updating fields as
// appropriate.
func MarshalJob(job quartz.ScheduledJob) (*ScheduledJob, error) {
	// Extract serialized job as models.ScheduledJob
	serialized, ok := job.JobDetail().Job().(*ScheduledJob)
	if !ok {
		return nil, errors.Join(ErrMarshalJobFailed, ErrInvalidJob)
	}

	// Update job values.
	serialized.JobNextRun = time.Unix(0, job.NextRunTime())
	serialized.CreatedAt = time.Now().UTC()
	serialized.JobOptions = job.JobDetail().Options()

	return serialized, nil
}
