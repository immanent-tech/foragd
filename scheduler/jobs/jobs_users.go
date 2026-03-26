// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/reugn/go-quartz/quartz"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/resend"
)

const (
	jobTypeNewInactiveUser = "new_inactive_user"
)

type newInactiveUserJob struct {
	*ScheduledJob
}

var _ quartz.Job = (*newInactiveUserJob)(nil)

type newInactiveUserJobData struct {
	UserID models.UserID `json:"user_id" validate:"required,starts_with=user_"`
}

// NewInactiveUserJob is a one-shot job to send an email to a new user who hasn't yet logged in reminding and asking
// them about the app.
func NewInactiveUserJob(userID models.UserID) (*newInactiveUserJob, error) {
	job := &newInactiveUserJob{
		ScheduledJob: &ScheduledJob{
			CreatedAt:      time.Now().UTC(),
			JobTriggerType: jobTriggerTypePoll,
			JobType:        jobTypeNewInactiveUser,
			JobDescription: "Send email to new but inactive user.",
		},
	}

	// Add trigger two days from now.
	trigger, err := json.Marshal(oneShotTrigger{Delay: 2 * 24 * time.Hour})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateJobFailed, err)
	}
	job.ScheduledJob.JobTrigger = trigger

	// Create job data.
	data, err := json.Marshal(newInactiveUserJobData{UserID: userID})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateJobFailed, err)
	}
	job.JobData = data

	return job, nil
}

func (j *newInactiveUserJob) Execute(ctx context.Context) error {
	// Get user details from job data.
	var data newInactiveUserJobData
	if err := json.Unmarshal(j.JobData, &data); err != nil {
		return fmt.Errorf("unmarshal job data: %w", err)
	}

	// Get user details.
	user, err := models.GetUser(ctx, data.UserID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}

	// If the user has not yet logged in.
	if user.LoginCount <= 1 {
		// Create and send email to user.
		email, err := resend.NewTemplatedEmail(
			"new-inactive-user",
			resend.To(user.GetEmail()),
			resend.WithTag(resend.TagCategory, resend.TagCategoryPromotional),
			resend.WithVariable("USER_NICKNAME", user.GetNickname()),
		)
		if err != nil {
			return fmt.Errorf("create new inactive user email: %w", err)
		}
		if err := resend.SendEmail(ctx, email); err != nil {
			return fmt.Errorf("send new inactive user email: %w", err)
		}
	}

	return nil
}

func (j *newInactiveUserJob) JobDetail() *quartz.JobDetail {
	var (
		data newInactiveUserJobData
		id   string
	)
	if err := json.Unmarshal(j.JobData, &data); err != nil {
		id, _ = gonanoid.New()
	} else {
		id = data.UserID
	}

	return quartz.NewJobDetail(j, j.generateJobKey(jobTypeNewInactiveUser+"_"+id, jobTypeNewInactiveUser))
}

func (j *newInactiveUserJob) AsScheduledJob() *ScheduledJob {
	return j.ScheduledJob
}
