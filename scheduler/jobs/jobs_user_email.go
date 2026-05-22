// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/resend"
	"github.com/immanent-tech/foragd/service"
)

// NewUserEmailJob create a one-shot job to email the user an app tip after a delay as part of onboarding/retention.
func NewUserEmailJob(
	userID models.UserID,
	emailTemplateID models.EmailTemplateID,
	delay time.Duration,
) (*SerializedJob, error) {
	// Create the update feed job.
	job := &SerializedJob{
		CreatedAt:      time.Now().UTC(),
		JobDescription: new("Send user tip email: " + userID + ": " + string(emailTemplateID) + ")"),
		JobKey:         quartz.NewJobKeyWithGroup(string(emailTemplateID), string(JobTypeUserEmailJob)).String(),
		JobType:        JobTypeUserEmailJob,
		JobNextRun:     models.UnixEpoch,
		JobTriggerType: TriggerTypeOneshot,
	}
	if err := job.JobData.FromUserEmailJob(UserEmailJob{UserID: userID, EmailId: emailTemplateID}); err != nil {
		return nil, fmt.Errorf("create job data: %w", err)
	}
	if err := job.JobTrigger.FromOneShotTrigger(OneShotTrigger{Delay: delay}); err != nil {
		return nil, fmt.Errorf("create trigger: %w", err)
	}

	return job, nil
}

func ExecuteUserEmail(ctx context.Context, job *SerializedJob) error {
	data, err := job.JobData.AsUserEmailJob()
	if err != nil {
		return fmt.Errorf("unable to unmarshal job data: %w", err)
	}

	start := time.Now()

	// Get user details.
	user, err := service.GetUser(ctx, data.UserID)
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
		string(data.EmailId),
		resend.WithTo(user.GetEmail()),
		resend.WithTag(resend.TagCategory, resend.TagCategoryPromotional),
		resend.WithVariable("USER_NICKNAME", nickname),
		resend.WithVariable("USER_UNSUBSCRIBE_LINK", "/unsubscribe/"+unsubscribeToken),
	)
	if err != nil {
		return fmt.Errorf("create new tip email %s: %w", data.EmailId, err)
	}
	if err := resend.SendEmail(ctx, resend.WithExistingEmail(email)); err != nil {
		return fmt.Errorf("send tip email %s: %w", data.EmailId, err)
	}

	slogctx.FromCtx(ctx).Debug("Finished user tips job.",
		slog.String("tip", string(data.EmailId)),
		slog.Duration("took", time.Since(start)))

	return nil
}
