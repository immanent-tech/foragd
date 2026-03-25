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
	jobTypeUserRetention = "user_retention"
	// jobStateUserNewInactive = jobTypeUserNewInactive + "_state"
)

// // UserNewInactiveState represents the state required by this job type.
// type UserNewInactiveState struct {
// 	// Checkpoint is the timestamp when the job last checked for new and inactive users.
// 	Checkpoint time.Time `json:"checkpoint"`
// }

type userRetentionJob struct {
	*ScheduledJob
}

// NewUserRetentionJob creates a job for pinging inactive new users.
func NewUserRetentionJob() (*userRetentionJob, error) {
	job := &userRetentionJob{
		ScheduledJob: &ScheduledJob{
			CreatedAt:      time.Now().UTC(),
			JobTriggerType: jobTriggerTypePoll,
			JobType:        jobTypeUserRetention,
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

func (j *userRetentionJob) Execute(ctx context.Context) error {

	// Find new inactive users and email them to check-in.
	if err := emailNewInactiveUsers(ctx); err != nil {
		slogctx.FromCtx(ctx).Warn("Email new inactive users failed.",
			slog.Any("error", err),
		)
	}

	return nil
}

func (j *userRetentionJob) JobDetail() *quartz.JobDetail {
	return quartz.NewJobDetail(j, j.generateJobKey(jobTypeUserRetention, ""))
}

func (j *userRetentionJob) AsScheduledJob() *ScheduledJob {
	return j.ScheduledJob
}

func emailNewInactiveUsers(ctx context.Context) error {
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
		return fmt.Errorf("find new inactive users: %w", err)
	}

	// Create emails to send to users who are inactive and haven't already been emailed.
	emails := make([]*resend.Email, 0, len(users))
	for u := range slices.Values(users) {
		user := u.UserResponseSchema
		if user.UserMetadata != nil {
			metadata := *user.UserMetadata
			if _, ok := metadata["new_inactive_email_sent"].(string); ok {
				continue
			}
		}

		// Send email to new inactive users.
		email, err := resend.NewTemplatedEmail(
			"new-inactive-user",
			resend.To(user.GetEmail()),
			resend.WithTag(resend.TagCategory, resend.TagCategoryPromotional),
			resend.WithVariable("USER_NICKNAME", user.GetNickname()),
		)
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Could not create email.",
				slog.String("auth0_id", user.GetUserID()),
				slog.Any("error", err),
			)
			continue
		}

		emails = append(emails, email)
	}

	// Send out the emails.
	resp, err := resend.BatchSendEmails(ctx, emails...)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Errors occurred sending batch emails.",
			slog.Any("error", err),
		)
	}

	failedEmails := resp.GetFailed()

	// Update user metadata for successful requests.
	for u := range slices.Values(users) {
		if user := u.UserResponseSchema; !slices.Contains(slices.Collect(maps.Keys(failedEmails)), user.GetEmail()) {
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
	}

	// Log failed requests.
	for email, err := range failedEmails {
		if idx := slices.IndexFunc(users, func(e *auth0.UserData) bool {
			return e.UserResponseSchema.GetEmail() == email
		}); idx != -1 {
			slogctx.FromCtx(ctx).Error("Failed to ping new inactive user.",
				slog.String("user_id", users[idx].UserResponseSchema.GetUserID()),
				slog.Any("error", err),
			)
		}
	}

	return nil
}
