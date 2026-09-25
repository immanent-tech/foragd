/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package jobs

import (
	"context"

	"github.com/go-resty/resty/v2"
	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

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
)

type contextKey string

type SchedulerAPI interface {
	GetScheduledJob(jobKey *quartz.JobKey) (quartz.ScheduledJob, error)
	ScheduleJob(jobDetail *quartz.JobDetail, trigger quartz.Trigger) error
	DeleteJob(jobKey *quartz.JobKey) error
	PauseJob(jobKey *quartz.JobKey) error
	GetJobKeys(...quartz.Matcher[quartz.ScheduledJob]) ([]*quartz.JobKey, error)
}

func SchedulerAPIToCtx(ctx context.Context, schedulerAPI SchedulerAPI) context.Context {
	return context.WithValue(ctx, schedulerAPICtxKey, schedulerAPI)
}

func SchedulerAPIFromCtx(ctx context.Context) SchedulerAPI {
	if api, ok := ctx.Value(schedulerAPICtxKey).(SchedulerAPI); ok {
		return api
	}
	slogctx.Warn(ctx, "No scheduler api in context.")
	return nil
}

func IndexerToCtx(ctx context.Context, indexer *bulk.Indexer) context.Context {
	return context.WithValue(ctx, indexerCtxKey, indexer)
}

func IndexerFromCtx(ctx context.Context) (*bulk.Indexer, bool) {
	indexer, ok := ctx.Value(indexerCtxKey).(*bulk.Indexer)
	return indexer, ok
}

func HTTPClientToCtx(ctx context.Context, httpClient *resty.Client) context.Context {
	return context.WithValue(ctx, httpClientCtxKey, httpClient)
}

func HTTPClientFromCtx(ctx context.Context) *resty.Client {
	if httpClient, ok := ctx.Value(httpClientCtxKey).(*resty.Client); ok {
		return httpClient
	}
	slogctx.Warn(ctx, "No http client found in context, using default client")
	return resty.New()
}

func ItemCacheToCtx(ctx context.Context, itemCache cache.ObjectCache) context.Context {
	return context.WithValue(ctx, itemCacheCtxKey, itemCache)
}

func ItemCacheFromCtx(ctx context.Context) cache.ObjectCache {
	if itemCache, ok := ctx.Value(itemCacheCtxKey).(cache.ObjectCache); ok {
		return itemCache
	}
	slogctx.Warn(ctx, "No item cache found in context")
	return nil
}

func ElasticToCtx(ctx context.Context, svc *service.ElasticService) context.Context {
	return context.WithValue(ctx, elasticCtxKey, svc)
}

func ElasticFromCtx(ctx context.Context) *service.ElasticService {
	if svc, ok := ctx.Value(elasticCtxKey).(*service.ElasticService); ok {
		return svc
	}
	slogctx.Warn(ctx, "No elastic service found in context")
	return nil
}

func UserSvcToCtx(ctx context.Context, svc *service.UserService) context.Context {
	return context.WithValue(ctx, userSvcCtxKey, svc)
}

func UserSvcFromCtx(ctx context.Context) *service.UserService {
	if svc, ok := ctx.Value(userSvcCtxKey).(*service.UserService); ok {
		return svc
	}
	slogctx.Warn(ctx, "No user service found in context")
	return nil
}

func FeedSvcToCtx(ctx context.Context, svc *service.FeedService) context.Context {
	return context.WithValue(ctx, feedSvcCtxKey, svc)
}

func FeedSvcFromCtx(ctx context.Context) *service.FeedService {
	if svc, ok := ctx.Value(feedSvcCtxKey).(*service.FeedService); ok {
		return svc
	}
	slogctx.Warn(ctx, "No feed service found in context")
	return nil
}

func ItemSvcToCtx(ctx context.Context, svc *service.ItemService) context.Context {
	return context.WithValue(ctx, itemSvcCtxKey, svc)
}

func ItemSvcFromCtx(ctx context.Context) *service.ItemService {
	if svc, ok := ctx.Value(itemSvcCtxKey).(*service.ItemService); ok {
		return svc
	}
	slogctx.Warn(ctx, "No feed service found in context")
	return nil
}

func ImportSvcToCtx(ctx context.Context, svc *service.ImportService) context.Context {
	return context.WithValue(ctx, importSvcCtxKey, svc)
}

func ImportSvcFromCtx(ctx context.Context) *service.ImportService {
	if svc, ok := ctx.Value(importSvcCtxKey).(*service.ImportService); ok {
		return svc
	}
	slogctx.Warn(ctx, "No import service found in context")
	return nil
}
