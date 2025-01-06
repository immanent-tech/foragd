// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/hibiken/asynq"

	"github.com/joshuar/go-feed-me/internal/logging"
)

const (
	DefaultWorkerConcurrency = 10
)

func NewTaskWorker(ctx context.Context, cache Cache, db DB) error {
	if err := loadConfig(); err != nil {
		return fmt.Errorf("cannot start scheduler: %w", err)
	}

	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr: config.RedisServer + ":" + strconv.Itoa(config.RedisPort),
		},
		asynq.Config{Concurrency: DefaultWorkerConcurrency},
	)

	itemsWorker := &TaskRunner{
		db:     db,
		cache:  cache,
		logger: logging.FromContext(ctx).WithGroup("worker"),
	}

	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeGetFeedItems, itemsWorker.HandleGetFeedItemsTask)

	go func() {
		if err := srv.Run(mux); err != nil {
			logging.FromContext(ctx).Error("Worker could not start.", slog.Any("error", err))
		}
	}()

	go func() {
		defer srv.Shutdown()
		<-ctx.Done()
	}()

	return nil
}
