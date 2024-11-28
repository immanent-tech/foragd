// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package scheduler

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/hibiken/asynq"
	"github.com/knadh/koanf/v2"

	"github.com/joshuar/go-feed-me/internal/logging"
)

const (
	DefaultWorkerConcurrency = 10
)

func NewTaskWorker(ctx context.Context, config *koanf.Koanf, cache Cache, db DB) {
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
