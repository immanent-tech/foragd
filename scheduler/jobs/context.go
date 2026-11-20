/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package jobs

import (
	"context"
	"time"

	"github.com/reugn/go-quartz/quartz"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/resend"
)

const (
	servicesCtxKey contextKey = "services"
)

type contextKey string

type SchedulerAPI interface {
	GetScheduledJob(jobKey *quartz.JobKey) (quartz.ScheduledJob, error)
	ScheduleJob(jobDetail *quartz.JobDetail, trigger quartz.Trigger) error
	DeleteJob(jobKey *quartz.JobKey) error
	PauseJob(jobKey *quartz.JobKey) error
	UpdateJob(ctx context.Context, job *SerializedJob) error
	GetJobKeys(...quartz.Matcher[quartz.ScheduledJob]) ([]*quartz.JobKey, error)
	ClearDeletedFeedJobs(ctx context.Context) error
}

type FeedsAPI interface {
	GetFeeds(ctx context.Context, ids ...models.FeedID) (models.Feeds, error)
	GetNewFeedsSince(ctx context.Context, since time.Time) (models.Feeds, error)
	GetAllFeedsExcept(ctx context.Context, feedIDs ...models.FeedID) (models.Feeds, error)
	GetFeed(ctx context.Context, feedID models.FeedID) (*models.Feed, error)
	FetchFeedUpdates(ctx context.Context, details models.Feed) (*models.Feed, models.URL, error)
	FetchFeedUpdatesAsArticles(ctx context.Context, details models.Feed) (*models.Feed, models.URL, error)
	ApplyFeedUpdates(ctx context.Context, oldData, newData *models.Feed) error
	UpdateFeed(ctx context.Context, feed *models.Feed) error
	Close() error
}

type ImportAPI interface {
	GetPendingImports(ctx context.Context) ([]*models.ImportStatus, error)
	ProcessRequests(ctx context.Context, status *models.ImportStatus) error
	Close() error
}

type UserAPI interface {
	GetUser(ctx context.Context, userID models.UserID) (*models.User, error)
	Close() error
}

type EmailSender interface {
	Send(ctx context.Context, options ...resend.EmailOption) error
}

type Services struct {
	Scheduler   SchedulerAPI
	Feeds       FeedsAPI
	Imports     ImportAPI
	Users       UserAPI
	EmailSender EmailSender
}

func ServicesToCtx(ctx context.Context, services *Services) context.Context {
	return context.WithValue(ctx, servicesCtxKey, services)
}

func ServicesFromCtx(ctx context.Context) *Services {
	if services, ok := ctx.Value(servicesCtxKey).(*Services); ok {
		return services
	}
	return nil
}
