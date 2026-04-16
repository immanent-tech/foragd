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
	jobTypeUserTips = "user_tips_job"
)

type userTipsJob struct {
	*ScheduledJob
}

var _ quartz.Job = (*userTipsJob)(nil)

var UserTipsJobs = map[string]time.Duration{
	"tip-email-newsletters": 24 * time.Hour,
	"new-inactive-user":     5 * 24 * time.Hour,
}

type userTipsJobData struct {
	EmailID string        `json:"email_id" validate:"required"`
	UserID  models.UserID `json:"user_id"  validate:"required,starts_with=user_"`
}

// NewUserTipsJob create a one-shot job to email the user an app tip after a delay as part of onboarding/retention.
func NewUserTipsJob(userID models.UserID, tip string) (*userTipsJob, error) {
	job := &userTipsJob{
		ScheduledJob: &ScheduledJob{
			CreatedAt:      time.Now().UTC(),
			JobTriggerType: jobTriggerTypeOneShot,
			JobType:        jobTypeUserTips,
			JobDescription: "Send tips to user via email.",
		},
	}

	// Add trigger two days from now.
	trigger, err := json.Marshal(oneShotTrigger{Delay: UserTipsJobs[tip]})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateJobFailed, err)
	}
	job.ScheduledJob.JobTrigger = trigger

	// Create job data.
	data, err := json.Marshal(userTipsJobData{UserID: userID, EmailID: tip})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateJobFailed, err)
	}
	job.JobData = data

	return job, nil
}

func (j *userTipsJob) Execute(ctx context.Context) error {
	// Get user details from job data.
	var data userTipsJobData
	if err := json.Unmarshal(j.JobData, &data); err != nil {
		return fmt.Errorf("unmarshal job data: %w", err)
	}

	// Get user details.
	user, err := models.GetUser(ctx, data.UserID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}

	// Don't send tips if user has requested not to receive promotional emails.
	if !user.Metadata.PromotionalEmail {
		return nil
	}

	// Generate token for unsubscribe link.
	unsubscribeToken, err := resend.EncodeEmail(user.GetEmail())
	if err != nil {
		return fmt.Errorf("generate unsubscribe token: %w", err)
	}

	nickname := user.GetNickname()
	if nickname == "" {
		nickname = "there"
	}

	// Create and send email to user.
	email, err := resend.NewTemplatedEmail(
		data.EmailID,
		resend.To(user.GetEmail()),
		resend.WithTag(resend.TagCategory, resend.TagCategoryPromotional),
		resend.WithVariable("USER_NICKNAME", nickname),
		resend.WithVariable("USER_UNSUBSCRIBE_LINK", "/unsubscribe/"+unsubscribeToken),
	)
	if err != nil {
		return fmt.Errorf("create new tip email %s: %w", data.EmailID, err)
	}
	if err := resend.SendEmail(ctx, resend.WithExistingEmail(email)); err != nil {
		return fmt.Errorf("send tip email %s: %w", data.EmailID, err)
	}

	return nil
}

func (j *userTipsJob) JobDetail() *quartz.JobDetail {
	var (
		data userTipsJobData
		id   string
		tip  string
	)
	if err := json.Unmarshal(j.JobData, &data); err != nil {
		id, _ = gonanoid.New()
		tip, _ = gonanoid.New()
	} else {
		id = data.UserID
		tip = data.EmailID
	}

	return quartz.NewJobDetail(j, j.GenerateJobKey(jobTypeUserTips+"_"+id+"_"+tip, jobTypeUserTips))
}

func (j *userTipsJob) AsScheduledJob() *ScheduledJob {
	return j.ScheduledJob
}
