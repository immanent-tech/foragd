// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
	for u := range slices.Values(users) {
		user := u.UserResponseSchema
		if user.UserMetadata != nil {
			metadata := *user.UserMetadata
			if _, ok := metadata["new_inactive_email_sent"].(string); ok {
				continue
			}
		}

		// Send email to new inactive users.
		if err := resend.SendTemplatedEmail(
			ctx,
			"new-inactive-user",
			resend.To(user.GetEmail()),
			resend.WithTemplateVariable("USER_NICKNAME", user.GetNickname()),
		); err != nil {
			return fmt.Errorf("email new inactive users: %w", err)
		}

		// Update the user metadata.
		if err := auth0.UpdateUserMetadata(
			ctx,
			user.GetUserID(),
			"new_inactive_email_sent",
			time.Now().UTC(),
		); err != nil {
			slogctx.FromCtx(ctx).Warn("Could not update inactive user.",
				slog.String("user_id", user.GetUserID()),
				slog.Any("error", err),
			)
		} else {
			slogctx.FromCtx(ctx).Info("Pinged new inactive user.",
				slog.String("user_id", user.GetUserID()),
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
