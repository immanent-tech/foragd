// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/providers/resend"
)

const (
	jobTypeUserNewInactive = "user_new_inactive"
	// jobStateUserNewInactive = jobTypeUserNewInactive + "_state"
)

// // UserNewInactiveState represents the state required by this job type.
// type UserNewInactiveState struct {
// 	// Checkpoint is the timestamp when the job last checked for new and inactive users.
// 	Checkpoint time.Time `json:"checkpoint"`
// }

type inactiveNewUsersJob struct {
	*ScheduledJob
}

// NewUserNewInactiveJob creates a job for pinging inactive new users.
func NewUserNewInactiveJob() (*inactiveNewUsersJob, error) {
	job := &inactiveNewUsersJob{
		ScheduledJob: &ScheduledJob{
			CreatedAt:      time.Now().UTC(),
			JobTriggerType: jobTriggerTypePoll,
			JobType:        jobTypeUserNewInactive,
			JobDescription: "Check for new users who are inactive.",
		},
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

func (j *inactiveNewUsersJob) Execute(ctx context.Context) error {
	// newInactiveQuery := query.Bool(
	// 	query.Filter(
	// 		// Account is older than two days.
	// 		query.Before("created_at", time.Now().UTC().Add(-48*time.Hour)),
	// 		// Number of logins is <= 1.
	// 		query.NumberRange("login_count", query.IntLessThan(2)),
	// 	),
	// )
	// 	_, err := elastic.SearchAll[*models.User](
	// 	ctx,
	// 	schema.UsersIndexRO,
	// 	newInactiveQuery,
	// 	models.DefaultPaginationSize,
	// )
	// if err != nil {
	// 	return fmt.Errorf("search users: %w", err)
	// }

	// Get inactive users in backend.
	users, err := auth0.GetNewInactiveUsers(ctx)
	if err != nil {
		panic(err)
	}

	// Check for a ping email sent already, otherwise add to a map of users to ping.
	inactiveUsers := make(map[string]string)
	for user := range slices.Values(users) {
		if user.UserResponseSchema.UserMetadata != nil {
			metadata := *user.UserResponseSchema.UserMetadata
			if _, ok := metadata["new_inactive_email_sent"].(string); ok {
				continue
			}
		}
		inactiveUsers[user.UserResponseSchema.GetUserID()] = user.UserResponseSchema.GetEmail()
	}

	// Send email to new inactive users.
	if err := resend.SendTemplatedEmail(
		ctx,
		"new-inactive-user",
		resend.Bcc(slices.Collect(maps.Values(inactiveUsers))...),
	); err != nil {
		return fmt.Errorf("email new inactive users: %w", err)
	}

	// Update the user metadata.
	for id := range inactiveUsers {
		if err := auth0.UpdateUserMetadata(ctx, id, "new_inactive_email_sent", time.Now().UTC()); err != nil {
			slogctx.FromCtx(ctx).Warn("Could not update inactive user.",
				slog.String("user_id", id),
				slog.Any("error", err),
			)
		} else {
			slogctx.FromCtx(ctx).Info("Pinged new inactive user.",
				slog.String("user_id", id),
				slog.Any("error", err),
			)
		}
	}

	return nil
}

func (j *inactiveNewUsersJob) JobDetail() *quartz.JobDetail {
	return quartz.NewJobDetail(j, j.generateJobKey(jobTypeUserNewInactive, ""))
}

func (j *inactiveNewUsersJob) AsScheduledJob() *ScheduledJob {
	return j.ScheduledJob
}
