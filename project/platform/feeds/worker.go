// Copyright (C) 2024 Joshua Rich <joshua.rich@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package feeds

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/reugn/go-quartz/job"
	"github.com/reugn/go-quartz/quartz"

	"github.com/joshuar/go-feed-me/platform/scheduler"
)

type GetFeedsWorker struct {
	lastRun time.Time
	db      dbAPI
	cache   cacheAPI
}

func (w *GetFeedsWorker) Description() string {
	return "FeedWorker"
}

func (w *GetFeedsWorker) Execute(ctx context.Context) (int, error) {
	since := w.lastRun
	w.lastRun = time.Now()

	newFeeds, err := w.db.GetUpdatedFeedURLs(since)
	if err != nil {
		return 0, fmt.Errorf("cannot retrieve updated feeds: %w", err)
	}

	for _, feed := range newFeeds {
		slog.Debug("Creating job for feed.", slog.String("url", feed))

		w := newFeedWorker(feed, w.cache)
		feedJob := job.NewFunctionJobWithDesc(
			func(ctx context.Context) (int, error) { return w.Execute(ctx) },
			w.Description(),
		)
		functionJobDetail := quartz.NewJobDetail(feedJob, quartz.NewJobKey(feed))

		err := scheduler.Schedule(functionJobDetail, quartz.NewSimpleTrigger(time.Minute))
		if err != nil {
			return 0, fmt.Errorf("could not schedule job for feed %s: %w", feed, err)
		}
		w.Execute(ctx)
	}

	return len(newFeeds), nil
}

func NewGetFeedsWorker(ctx context.Context, db dbAPI, cache cacheAPI) error {
	worker := &GetFeedsWorker{
		db:    db,
		cache: cache,
	}

	quartzJob := job.NewFunctionJobWithDesc(
		func(ctx context.Context) (int, error) { return worker.Execute(ctx) },
		worker.Description(),
	)
	jobDetail := quartz.NewJobDetail(quartzJob, quartz.NewJobKey("feeds"))

	err := scheduler.Schedule(jobDetail, quartz.NewSimpleTrigger(time.Minute))
	if err != nil {
		return fmt.Errorf("could not schedule job for feeds: %w", err)
	}

	worker.Execute(ctx)

	slog.Debug("Started tracking feeds.")

	return nil
}

type FeedWorker struct {
	lastRun time.Time
	cache   cacheAPI
	url     string
}

func (j *FeedWorker) Description() string {
	return fmt.Sprintf("FeedItemWorker for %s", j.url)
}

func (j *FeedWorker) Execute(_ context.Context) (int, error) {
	// Update lastRun
	since := j.lastRun
	j.lastRun = time.Now()

	feed, err := FetchFeed(j.url)
	if err != nil {
		return 0, fmt.Errorf("cannot cache feed: %w", err)
	}

	itemCh := make(chan FeedItem)
	go j.cache.CacheFeedItems(itemCh)

	for _, item := range feed.GetItemsSince(since) {
		itemCh <- item
	}

	close(itemCh)

	return 0, nil
}

func newFeedWorker(url string, cache cacheAPI) *FeedWorker {
	return &FeedWorker{
		url:   url,
		cache: cache,
	}
}
