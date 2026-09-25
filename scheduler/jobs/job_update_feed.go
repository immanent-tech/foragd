/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/validation"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/service"
)

const updateFeedJobTimeout = 5 * time.Minute

var ErrFetchFailed = errors.New("fetching feed details failed")

// NewUpdateFeedJob creates a job for updating a feed.
func NewUpdateFeedJob(ctx context.Context, feedSvc *service.FeedService, id models.FeedID) (*SerializedJob, error) {
	// Get the feed details.
	feed, err := feedSvc.GetFeed(ctx, id)
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

	if err := validation.Validate.Struct(data); err != nil {
		return fmt.Errorf("validate job data: %w", err)
	}

	if data.Blocked {
		slogctx.Warn(ctx, "Not running blocked update feed job.",
			slog.String("feed_id", data.FeedID),
			slog.String("reason", *data.BlockedReason),
		)
		return nil
	}

	httpClient := HTTPClientFromCtx(ctx)
	itemsCache := ItemCacheFromCtx(ctx)

	feedSvc := FeedSvcFromCtx(ctx)
	if feedSvc == nil {
		return errors.New("cannot execute: no feed service in context")
	}

	itemSvc := ItemSvcFromCtx(ctx)
	if itemSvc == nil {
		return errors.New("cannot execute: no item service in context")
	}

	start := time.Now()

	ctx, cancel := context.WithTimeoutCause(ctx, updateFeedJobTimeout, errors.New("update feed timeout"))
	defer cancel()

	// Add feed id as slog attribute for log tracking.
	ctx = slogctx.With(ctx, "feed_id", data.FeedID)

	// Retrieve the feed details.
	details, err := feedSvc.GetFeed(ctx, data.FeedID)
	switch {
	case err != nil && errors.Is(err, elastic.ErrNotFound):
		return fmt.Errorf("cannot execute: %s: no feed found", data.FeedID)
	case err != nil:
		return fmt.Errorf("get feed doc: %s: %w", data.FeedID, err)
	}

	// Add additional feed details to logs.
	ctx = slogctx.With(ctx, "feed_name", details.GetTitle())

	slogctx.Debug(ctx, "Fetching latest feed source.")
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
		feed, feedURL, err = service.FetchFeedUpdates(ctx, httpClient, details)
	}
	if err != nil {
		return fmt.Errorf("fetch feed: %w", err)
	}

	// Record the feed URL used in the logs.
	ctx = slogctx.With(ctx, "feed_url", feedURL)

	slogctx.Debug(ctx, "Feed fetched. Applying updates.")

	if err := feedSvc.ApplyFeedUpdates(ctx, itemSvc, httpClient, itemsCache, details, feed); err != nil {
		slogctx.Error(ctx, "Could not apply feed updates.",
			slog.Any("error", err))
	}

	slogctx.Debug(ctx, "Finished update feed job.",
		slog.Duration("took", time.Since(start)))

	return nil
}
