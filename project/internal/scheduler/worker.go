// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package scheduler

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/hibiken/asynq"
	"github.com/knadh/koanf/v2"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
)

const (
	DefaultWorkerConcurrency = 10
)

type cacheAPI interface {
	CacheFeedItems(itemCh chan models.FeedItem)
}

type dbAPI interface {
	GetNewFeeds(since time.Time) ([]models.Feed, error)
	GetNewItems(feedID string) ([]models.FeedItem, error)
}

func NewTaskWorker(ctx context.Context, config *koanf.Koanf, db dbAPI, cache cacheAPI) {
	settings := getSettings(config)
	logger := logging.FromContext(ctx).WithGroup("tasks").With(slog.String("component", "worker"))

	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr: settings.RedisServer + ":" + strconv.Itoa(settings.RedisPort),
		},
		asynq.Config{Concurrency: DefaultWorkerConcurrency},
	)

	itemsWorker := &TaskRunner{
		db:     db,
		cache:  cache,
		logger: logger,
	}

	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeGetFeedItems, itemsWorker.HandleGetFeedItemsTask)

	go func() {
		if err := srv.Run(mux); err != nil {
			logger.Error("Worker could not start.", slog.Any("error", err))
		}
	}()

	go func() {
		defer srv.Shutdown()
		<-ctx.Done()
	}()
}
