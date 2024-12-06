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

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
)

const (
	DefaultTaskManagerSyncInterval = time.Minute
	DefaultTaskCron                = "* * * * *"
)

// DB represents the required methods that the scheduler needs from a database
// implementation.
type DB interface {
	GetFeedLastFetched(ctx context.Context, feedID string) (time.Time, error)
	UpdateFeedLastFetched(ctx context.Context, feedID string, lastFetched time.Time) error
}

// Cache represents the required methods that the scheduler needs from a cache
// implementation.
type Cache interface {
	GetNewFeedsSince(ctx context.Context, since time.Time) ([]models.APIFeed, error)
	AddFeedItems(ctx context.Context, items ...models.Item) error
}

type taskScheduler struct {
	logger      *slog.Logger
	cache       Cache
	checkpoint  time.Time
	taskConfigs []*asynq.PeriodicTaskConfig
}

func (s *taskScheduler) GetConfigs() ([]*asynq.PeriodicTaskConfig, error) {
	s.logger.Debug("Checking for new feeds.",
		slog.Time("since", s.checkpoint.UTC()))

	feeds, err := s.cache.GetNewFeedsSince(context.TODO(), s.checkpoint)
	if err != nil {
		return nil, fmt.Errorf("cannot retrieve updated feeds: %w", err)
	}

	s.checkpoint = time.Now().UTC()

	for _, feed := range feeds {
		feedTask, err := NewGetFeedItemsTask(feed)
		if err != nil {
			s.logger.Error("Could not create new GetFeedItems task.",
				slog.String("feed_id", feed.ID),
				slog.String("feed_title", feed.Title),
				slog.Any("error", err))

			continue
		}

		slog.Debug("Adding task for feed.",
			slog.String("feed_id", feed.ID),
			slog.String("feed_title", feed.Title))

		s.taskConfigs = append(s.taskConfigs, &asynq.PeriodicTaskConfig{Cronspec: DefaultTaskCron, Task: feedTask})
	}

	return s.taskConfigs, nil
}

func NewTaskScheduler(ctx context.Context, cache Cache) error {
	if err := loadConfig(); err != nil {
		return fmt.Errorf("cannot start scheduler: %w", err)
	}

	logger := logging.FromContext(ctx).WithGroup("tasks").With(slog.String("component", "scheduler"))

	manager := &taskScheduler{
		logger:     logger,
		cache:      cache,
		checkpoint: time.Time{},
	}

	opts := asynq.PeriodicTaskManagerOpts{
		PeriodicTaskConfigProvider: manager,
		RedisConnOpt: asynq.RedisClientOpt{
			Addr: config.RedisServer + ":" + strconv.Itoa(config.RedisPort),
		},
		SyncInterval: DefaultTaskManagerSyncInterval,
	}

	logger.Debug("Starting task scheduler.",
		slog.Duration("sync_interval", opts.SyncInterval))

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
