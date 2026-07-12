// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package jobs

import (
	"context"

	"github.com/reugn/go-quartz/quartz"

	"github.com/immanent-tech/foragd/providers/elastic/bulk"
)

const (
	schedulerAPICtxKey contextKey = "scheduler_api"
	indexerCtxKey      contextKey = "indexer"
)

type contextKey string

type SchedulerAPI interface {
	GetScheduledJob(jobKey *quartz.JobKey) (quartz.ScheduledJob, error)
	ScheduleJob(jobDetail *quartz.JobDetail, trigger quartz.Trigger) error
	DeleteJob(jobKey *quartz.JobKey) error
	PauseJob(jobKey *quartz.JobKey) error
}

func SchedulerAPIToCtx(ctx context.Context, schedulerAPI SchedulerAPI) context.Context {
	return context.WithValue(ctx, schedulerAPICtxKey, schedulerAPI)
}

func IndexerToCtx(ctx context.Context, indexer *bulk.Indexer) context.Context {
	return context.WithValue(ctx, indexerCtxKey, indexer)
}

func IndexerFromCtx(ctx context.Context) (*bulk.Indexer, bool) {
	indexer, ok := ctx.Value(indexerCtxKey).(*bulk.Indexer)
	return indexer, ok
}
