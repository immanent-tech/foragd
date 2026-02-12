// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
)

// ClearExpiredSessionsState represents the state required by this job type.
type ClearExpiredSessionsState struct {
	// Checkpoint is the timestamp when the job last cleared expired sessions.
	Checkpoint time.Time `json:"checkpoint"`
}

// NewClearExpiredSessionsJob creates a job for checking for new feeds.
func NewClearExpiredSessionsJob() (*ScheduledJob, error) {
	job := &ScheduledJob{
		CreatedAt:      time.Now().UTC(),
		JobTriggerType: jobTriggerTypePoll,
		JobType:        jobTypeClearExpiredSessions,
		JobDescription: "Clear expired sessions.",
	}

	var (
		data []byte
		err  error
	)

	data, err = json.Marshal(newPollTrigger(24*time.Hour, time.Hour)) //nolint:mnd // Job trigger is ~every 24 hours.
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateJobFailed, err)
	}
	job.JobTrigger = data

	return job, nil
}

func executeClearExpiredSessions(ctx context.Context) error {
	jobStateID := "clear_expired_sessions_state"

	schedulerAPI, ok := ctx.Value(schedulerAPICtxKey).(SchedulerAPI)
	if !ok || schedulerAPI == nil {
		return errors.New("unable to get scheduler api from context")
	}

	// state := &ClearExpiredSessionsState{}
	// if lastState, err := schedulerAPI.GetJobState(ctx, jobStateID); err != nil {
	// 	if !errors.Is(err, elastic.ErrNotFound) {
	// 		return fmt.Errorf("get job state: %w", err)
	// 	}
	// 	state.Checkpoint = time.Time{}
	// } else {
	// 	err = json.Unmarshal(lastState.JobData, state)
	// 	if err != nil {
	// 		return fmt.Errorf("unmarshal job data: %w", err)
	// 	}
	// }

	// Delete all sessions with an expiry older than now.
	if err := elastic.DeleteDocs(
		ctx,
		schema.SessionsIndexRW,
		query.Before("expiry", time.Now().UTC()),
	); err != nil {
		return fmt.Errorf("delete docs: %w", err)
	}
	// Update the checkpoint.
	state := &ClearExpiredSessionsState{
		Checkpoint: time.Now().UTC(),
	}
	if err := schedulerAPI.UpdateJobState(ctx, jobStateID, map[string]any{
		"job_data": state,
	}); err != nil {
		return fmt.Errorf("update job state: %w", err)
	}

	return nil
}
