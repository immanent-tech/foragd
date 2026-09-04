// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	"github.com/immanent-tech/foragd/service"
)

var ErrFetchFailed = errors.New("fetching feed details failed")

// NewUpdateFeedJob creates a job for updating a feed.
func NewUpdateFeedJob(ctx context.Context, id models.FeedID) (*SerializedJob, error) {
	// Get the feed details.
	feed, err := service.GetFeed(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get feed: %w", err)
	}

	trigger := NewPollTrigger(feed.GetUpdateInterval(), DefaultPollJitter)

	// Create the update feed job.
	job := &SerializedJob{
		CreatedAt:      time.Now().UTC(),
		JobDescription: new("Update feed: " + feed.GetTitle() + " (" + id + ")"),
		JobKey:         quartz.NewJobKeyWithGroup(id, string(JobTypeUpdateFeed)).String(),
		JobType:        JobTypeUpdateFeed,
		JobNextRun:     models.UnixEpoch,
		JobTriggerType: TriggerTypePoll,
	}
	if err := job.JobData.FromUpdateFeedJob(UpdateFeedJob{FeedID: id, Deleted: false}); err != nil {
		return nil, fmt.Errorf("create job data: %w", err)
	}
	if err := job.JobTrigger.FromPollTrigger(*trigger); err != nil {
		return nil, fmt.Errorf("create trigger: %w", err)
	}

	return job, nil
}

// ExecuteUpdateFeed will execute a job that attempts to find new items for a feed and index them into the data backend.
func ExecuteUpdateFeed(ctx context.Context, job *SerializedJob) error {
	data, err := job.JobData.AsUpdateFeedJob()
	if err != nil {
		return fmt.Errorf("unable to unmarshal job data: %w", err)
	}

	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, defaultJobTimeout)
	defer cancel()

	// Add feed id as slog attribute for log tracking.
	ctx = slogctx.With(ctx, "feed_id", data.FeedID)

	// Retrieve the feed details.
	details, err := service.GetFeed(ctx, data.FeedID)
	switch {
	case err != nil && errors.Is(err, elastic.ErrNotFound):
		// If the returned error indicates there is no feed with the given ID, mark the job to be deleted.
		data.Deleted = true
		if marshalErr := job.JobData.FromUpdateFeedJob(data); marshalErr != nil {
			return fmt.Errorf("update job data: %w", marshalErr)
		}
		if bulkErr := bulk.AddAction(ctx,
			bulk.NewAction(
				job,
				bulk.AsOperation[string](bulk.OpIndex),
				bulk.ToIndex[string](schema.SchedulerIndexRW()),
			),
		); bulkErr != nil {
			return fmt.Errorf("update feed: %w", bulkErr)
		}
		if flushErr := bulk.Flush(ctx); flushErr != nil {
			return fmt.Errorf("update feed job: %w", flushErr)
		}
		slogctx.FromCtx(ctx).Warn("No feed found with that ID. Marking update feed job for deletion.")
		return nil
	case err != nil:
		return fmt.Errorf("get feed doc: %w", err)
	}

	// Add additional feed details to logs.
	ctx = slogctx.With(ctx, "feed_name", details.GetTitle())

	// Get new feed data.
	var (
		feed    *models.Feed
		feedURL string
	)
	switch details.FetchMethod {
	case models.FeedFetchMethodZyteArticles:
		// Zyte article list extraction.
		feed, feedURL, err = service.FetchFeedUpdatesAsArticles(ctx, details)
	case models.FeedFetchMethodDirect, models.FeedFetchMethodProxied:
		// Direct (or proxied) request.
		fallthrough
	default:
		// Assume a regular web-based feed. Fetch feed data directly.
		feed, feedURL, err = service.FetchFeedUpdates(ctx, details)
	}
	if err != nil {
		return fmt.Errorf("fetch feed: %w", err)
	}

	// Record the feed URL used in the logs.
	ctx = slogctx.With(ctx, "feed_url", feedURL)

	if err := service.ApplyFeedUpdates(ctx, details, feed); err != nil {
		slogctx.Error(ctx, "Could not apply feed updates.",
			slog.Any("error", err))
	}

	slogctx.FromCtx(ctx).Debug("Finished update feed job.",
		slog.Duration("took", time.Since(start)))

	return nil
}
