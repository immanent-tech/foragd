// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
)

var (
	ErrExecuteJobFailed = errors.New("could not execute job")
	ErrCreateJobFailed  = errors.New("create job failed")
	ErrMarshalJobFailed = errors.New("could not marshal job")
	ErrInvalidJob       = errors.New("invalid job")
	ErrNoJob            = errors.New("no job found")
)

const (
	defaultJobTimeout = 2 * time.Minute

	schedulerAPICtxKey contextKey = "scheduler_api"
)

type contextKey string

type SchedulerAPI interface {
	GetScheduledJob(jobKey *quartz.JobKey) (quartz.ScheduledJob, error)
	ScheduleJob(jobDetail *quartz.JobDetail, trigger quartz.Trigger) error
	DeleteJob(jobKey *quartz.JobKey) error
}

func SchedulerAPIToCtx(ctx context.Context, schedulerAPI SchedulerAPI) context.Context {
	return context.WithValue(ctx, schedulerAPICtxKey, schedulerAPI)
}

var _ quartz.ScheduledJob = (*SerializedJob)(nil)

func (j *SerializedJob) JobDetail() *quartz.JobDetail {
	return quartz.NewJobDetail(j, j.getJobKey())
}

func (j *SerializedJob) Trigger() quartz.Trigger {
	switch j.JobTriggerType {
	case TriggerTypePoll:
		if trigger, err := j.JobTrigger.AsPollTrigger(); err == nil {
			return &trigger
		}
	case TriggerTypeOneshot:
		if trigger, err := j.JobTrigger.AsOneShotTrigger(); err == nil {
			return &trigger
		}
	}
	return nil
}

func (j *SerializedJob) NextRunTime() int64 {
	return j.JobNextRun.UnixNano()
}

func (j *SerializedJob) getJobKey() *quartz.JobKey {
	jobKeyVals := strings.Split(j.JobKey, quartz.Sep)
	if jobKeyVals[1] != "" {
		return quartz.NewJobKeyWithGroup(jobKeyVals[1], jobKeyVals[0])
	}
	return quartz.NewJobKey(jobKeyVals[0])
}

var _ quartz.Job = (*SerializedJob)(nil)

func (j *SerializedJob) Description() string {
	if j.JobDescription != nil {
		return *j.JobDescription
	}
	return ""
}

func (j *SerializedJob) Execute(ctx context.Context) error {
	// Check whether the job should execute and bail early if required.
	ok, err := j.shouldExecute(ctx)
	if err != nil {
		return fmt.Errorf("should execute: %w", err)
	}
	if !ok {
		return nil
	}

	// Run the appropriate execution method for the job.
	slogctx.FromCtx(ctx).Debug("Running execution method for job.",
		slog.String("job_key", j.JobKey),
		slog.String("job_description", *j.JobDescription),
	)
	switch j.JobType {
	case JobTypeGetNewFeeds:
		return ExecuteGetNewFeeds(ctx, j)
	case JobTypeUpdateFeed:
		return ExecuteUpdateFeed(ctx, j)
	case JobTypeDeleteExpiredSessions:
		return ExecuteDeleteExpiredSessions(ctx, j)
	case JobTypeClearDeletedFeeds:
		return ExecuteClearDeletedFeeds(ctx, j)
	case JobTypeUserEmailJob:
		return ExecuteUserEmail(ctx, j)
	}

	// Fail if we can't find an execution method (i.e., not implemented).
	slogctx.FromCtx(ctx).Warn("Could not determine execution method for job.",
		slog.String("job_key", j.JobKey),
		slog.String("job_description", *j.JobDescription),
	)
	return nil
}

// shouldExecute will perform some additional logic on jobs for some triggers like oneshot which can expire. It returns
// a bool indicating whether the job should run and a non-nil error if one occurred.
func (j *SerializedJob) shouldExecute(ctx context.Context) (bool, error) {
	switch j.JobTriggerType { //nolint:gocritic // leave for future switch expansion.
	case TriggerTypeOneshot:
		trigger, err := j.JobTrigger.AsOneShotTrigger()
		if err != nil {
			return false, fmt.Errorf("unmarshal trigger: %w", err)
		}
		if trigger.Expired {
			return false, nil
		}
		trigger.Expired = true
		if err = j.JobTrigger.FromOneShotTrigger(trigger); err != nil {
			return false, fmt.Errorf("marshal trigger: %w", err)
		}
		// Update the job data (checkpoint).
		if err := elastic.UpdateDoc(
			ctx,
			schema.SchedulerIndexRW(),
			j.JobDetail().JobKey().String(),
			j,
			elastic.WithDocAsUpsert(true),
			elastic.WithRefresh(true),
		); err != nil {
			return false, fmt.Errorf("update job: %w", err)
		}
	}
	return true, nil
}
