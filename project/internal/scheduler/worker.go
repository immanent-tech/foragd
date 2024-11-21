// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/reugn/go-quartz/job"
	"github.com/reugn/go-quartz/quartz"

	"github.com/joshuar/go-feed-me/internal/models"
)

type dbAPI interface {
	GetUpdatedFeeds(since time.Time) ([]models.Feed, error)
}

type cacheAPI interface {
	CacheFeedItems(itemCh chan models.FeedItem)
}

type GetFeedsWorker struct {
	lastRun time.Time
}

//revive:disable:unused-receiver
func (w *GetFeedsWorker) Description() string {
	return "FeedWorker"
}

func (w *GetFeedsWorker) Execute(ctx context.Context, db dbAPI, cache cacheAPI) (int, error) {
	since := w.lastRun
	w.lastRun = time.Now()

	newFeeds, err := db.GetUpdatedFeeds(since)
	if err != nil {
		return 0, fmt.Errorf("cannot retrieve updated feeds: %w", err)
	}

	for _, feed := range newFeeds {
		slog.Debug("Creating job for feed.", slog.String("url", feed.URL))

		feedWorker := newFeedWorker(&feed)
		feedJob := job.NewFunctionJobWithDesc(
			func(ctx context.Context) (int, error) { return feedWorker.Execute(ctx, cache) },
			w.Description(),
		)
		functionJobDetail := quartz.NewJobDetail(feedJob, quartz.NewJobKey(feed.URL))

		if err := Schedule(functionJobDetail, quartz.NewSimpleTrigger(time.Minute)); err != nil {
			return 0, fmt.Errorf("could not schedule job for feed %s: %w", feed.ID, err)
		}

		if _, err := feedWorker.Execute(ctx, cache); err != nil {
			return 0, fmt.Errorf("could not execute feed worker %s: %w", feed.ID, err)
		}
	}

	return len(newFeeds), nil
}

func NewGetFeedsWorker(ctx context.Context, db dbAPI, cache cacheAPI) error {
	worker := &GetFeedsWorker{}

	quartzJob := job.NewFunctionJobWithDesc(
		func(ctx context.Context) (int, error) { return worker.Execute(ctx, db, cache) },
		worker.Description(),
	)
	jobDetail := quartz.NewJobDetail(quartzJob, quartz.NewJobKey("feeds"))

	err := Schedule(jobDetail, quartz.NewSimpleTrigger(time.Minute))
	if err != nil {
		return fmt.Errorf("could not schedule job for feeds: %w", err)
	}

	if _, err := worker.Execute(ctx, db, cache); err != nil {
		return fmt.Errorf("could not execute feed worker: %w", err)
	}

	slog.Debug("Started tracking feeds.")

	return nil
}

type FeedWorker struct {
	lastRun time.Time
	feed    *models.Feed
}

func (w *FeedWorker) Description() string {
	return fmt.Sprintf("FeedItemWorker for %s", w.feed.Title)
}

func (w *FeedWorker) Execute(_ context.Context, cacheAPI cacheAPI) (int, error) {
	// Update lastRun
	since := w.lastRun
	w.lastRun = time.Now()

	itemCh := make(chan models.FeedItem)
	go cacheAPI.CacheFeedItems(itemCh)

	for _, item := range w.feed.GetItemsSince(since) {
		itemCh <- item
	}

	close(itemCh)

	return 0, nil
}

func newFeedWorker(feed *models.Feed) *FeedWorker {
	return &FeedWorker{
		feed: feed,
	}
}
