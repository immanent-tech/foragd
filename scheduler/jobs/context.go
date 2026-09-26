/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package jobs

import (
	"context"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	"github.com/immanent-tech/foragd/server/cache"
	"github.com/immanent-tech/foragd/service"
)

const (
	schedulerAPICtxKey contextKey = "scheduler_api"
	indexerCtxKey      contextKey = "indexer"
	httpClientCtxKey   contextKey = "http_client"
	itemCacheCtxKey    contextKey = "item_cache"
	elasticCtxKey      contextKey = "elastic"
	userSvcCtxKey      contextKey = "user_svc"
	feedSvcCtxKey      contextKey = "feed_svc"
	itemSvcCtxKey      contextKey = "item_svc"
	importSvcCtxKey    contextKey = "import_svc"
	servicesCtxKey     contextKey = "services"
)

type contextKey string

type SchedulerAPI interface {
	GetScheduledJob(jobKey *quartz.JobKey) (quartz.ScheduledJob, error)
	ScheduleJob(jobDetail *quartz.JobDetail, trigger quartz.Trigger) error
	DeleteJob(jobKey *quartz.JobKey) error
	PauseJob(jobKey *quartz.JobKey) error
	GetJobKeys(...quartz.Matcher[quartz.ScheduledJob]) ([]*quartz.JobKey, error)
}

type FeedsAPI interface {
	GetNewFeedsSince(ctx context.Context, since time.Time) (models.Feeds, error)
	GetAllFeedsExcept(ctx context.Context, feedIDs ...models.FeedID) (models.Feeds, error)
	GetFeed(ctx context.Context, feedID models.FeedID) (*models.Feed, error)
	ApplyFeedUpdates(
		ctx context.Context,
		httpClient *resty.Client,
		itemsCache cache.ObjectCache,
		old, new *models.Feed,
	) error
	UpdateFeed(ctx context.Context, feed *models.Feed) error
}

type ImportAPI interface {
	GetPendingImports(ctx context.Context) ([]*models.ImportStatus, error)
	ProcessRequests(ctx context.Context, status *models.ImportStatus, httpClient *resty.Client) error
}

type UserAPI interface {
	GetUser(ctx context.Context, userID models.UserID) (*models.User, error)
}

type ElasticAPI interface {
	GetIndexRO(idx service.Index) string
	GetIndexRW(idx service.Index) string
}

type Services struct {
	Scheduler  SchedulerAPI
	Feeds      FeedsAPI
	Imports    ImportAPI
	Users      UserAPI
	Elastic    ElasticAPI
	ItemsCache cache.ObjectCache
	Indexer    *bulk.Indexer
	HttpClient *resty.Client
}

func ServicesToCtx(ctx context.Context, services *Services) context.Context {
	return context.WithValue(ctx, servicesCtxKey, services)
}

func ServicesFromCtx(ctx context.Context) *Services {
	if services, ok := ctx.Value(servicesCtxKey).(*Services); ok {
		return services
	}
	slogctx.Warn(ctx, "No services in context.")
	return nil
}
