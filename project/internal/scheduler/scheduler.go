// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/hibiken/asynq"
	"github.com/knadh/koanf/v2"

	"github.com/joshuar/go-feed-me/internal/logging"
)

const (
	DefaultTaskManagerSyncInterval = time.Minute
	DefaultTaskCron                = "* * * * *"
)

type taskScheduler struct {
	logger          *slog.Logger
	db              dbAPI
	lastConfigFetch time.Time
}

func (s *taskScheduler) GetConfigs() ([]*asynq.PeriodicTaskConfig, error) {
	s.logger.Debug("Fetching new task configs.", slog.Time("last_fetched", s.lastConfigFetch))

	feeds, err := s.db.GetNewFeeds(s.lastConfigFetch)
	if err != nil {
		return nil, fmt.Errorf("cannot retrieve updated feeds: %w", err)
	}

	s.lastConfigFetch = time.Now()

	var configs []*asynq.PeriodicTaskConfig

	for _, feed := range feeds {
		feedTask, err := NewGetFeedItemsTask(feed.ID)
		if err != nil {
			s.logger.Error("Could not create new GetFeedItems task.",
				slog.String("feed_id", feed.ID),
				slog.Any("error", err))

			continue
		}

		configs = append(configs, &asynq.PeriodicTaskConfig{Cronspec: DefaultTaskCron, Task: feedTask})
	}

	return configs, nil
}

func NewTaskScheduler(ctx context.Context, config *koanf.Koanf, db dbAPI) error {
	settings := getSettings(config)
	logger := logging.FromContext(ctx).WithGroup("tasks").With(slog.String("component", "scheduler"))

	manager := &taskScheduler{
		logger:          logger,
		db:              db,
		lastConfigFetch: time.Time{},
	}

	opts := asynq.PeriodicTaskManagerOpts{
		PeriodicTaskConfigProvider: manager,
		RedisConnOpt: asynq.RedisClientOpt{
			Addr: settings.RedisServer + ":" + strconv.Itoa(settings.RedisPort),
		},
		SyncInterval: DefaultTaskManagerSyncInterval,
	}

	logger.Debug("Starting task scheduler.", slog.Any("options", opts))

	scheduler, err := asynq.NewPeriodicTaskManager(opts)
	if err != nil {
		return fmt.Errorf("could not start task scheduler: %w", err)
	}

	go func() {
		if err := scheduler.Start(); err != nil {
			logger.Error("Scheduler could not start.", slog.Any("error", err))
		}
	}()

	go func() {
		defer scheduler.Shutdown()
		<-ctx.Done()
	}()

	return nil
}
