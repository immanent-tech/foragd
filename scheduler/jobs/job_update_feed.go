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
	"sync"
	"time"

	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/validation"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
)

const updateFeedJobTimeout = 15 * time.Minute

var ErrFetchFailed = errors.New("fetching feed details failed")

// TODO: this should be in the backing store for distributed checks.
var feedsInFlight sync.Map // feedID -> struct{}.

// NewUpdateFeedJob creates a job for updating a feed.
func NewUpdateFeedJob(ctx context.Context, feedSvc FeedsAPI, id models.FeedID) (*SerializedJob, error) {
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
	// Get the feed job data.
	data, err := job.JobData.AsUpdateFeedJob()
	if err != nil {
		return fmt.Errorf("unable to unmarshal job data: %w", err)
	}
	if err := validation.Validate.Struct(data); err != nil {
		return fmt.Errorf("validate job data: %w", err)
	}

	// Skip if this feed job has been manually blocked.
	if data.Blocked {
		reason := "unspecified"
		if data.BlockedReason != nil {
			reason = *data.BlockedReason
		}
		slogctx.Warn(ctx, "Skipping blocked feed.", slog.String("reason", reason))
		return nil
	}

	// Skip if there is an active job for this feed.
	if _, loaded := feedsInFlight.LoadOrStore(data.FeedID, struct{}{}); loaded {
		return nil
	}
	defer feedsInFlight.Delete(data.FeedID)

	services := ServicesFromCtx(ctx)
	if services == nil {
		return errors.New("no services in context")
	}

	start := time.Now()

	ctx, cancel := context.WithTimeoutCause(ctx, updateFeedJobTimeout, errors.New("update feed timeout"))
	defer cancel()

	// Add feed id as slog attribute for log tracking.
	ctx = slogctx.With(ctx, "feed_id", data.FeedID)

	// Retrieve the feed details.
	details, err := services.Feeds.GetFeed(ctx, data.FeedID)
	switch {
	case err != nil && errors.Is(err, elastic.ErrNotFound):
		if err := services.Scheduler.DeleteJob(job.getJobKey()); err != nil {
			slogctx.FromCtx(ctx).Error("Cannot remove job for missing feed", slog.Any("error", err))
		} // adapt to your job type
		return fmt.Errorf("cannot execute: %s: no feed found", data.FeedID)
	case err != nil:
		return fmt.Errorf("get feed doc: %s: %w", data.FeedID, err)
	}

	// Add additional feed details to logs.
	ctx = slogctx.With(ctx, "feed_name", details.GetTitle())

	slogctx.Info(ctx, "Running update feed job.")
	// Get new feed data.
	var (
		feed    *models.Feed
		feedURL string
	)
	switch details.FetchMethod {
	case models.FeedFetchMethodZyteArticles:
		// Zyte article list extraction.
		feed, feedURL, err = services.Feeds.FetchFeedUpdatesAsArticles(ctx, details)
	case models.FeedFetchMethodDirect, models.FeedFetchMethodProxied:
		// Direct (or proxied) request.
		fallthrough
	default:
		// Assume a regular web-based feed. Fetch feed data directly.
		feed, feedURL, err = services.Feeds.FetchFeedUpdates(ctx, details)
	}
	if err != nil {
		return fmt.Errorf("fetch feed: %w", err)
	}

	// Record the feed URL used in the logs.
	ctx = slogctx.With(ctx, "feed_url", feedURL)

	if err := services.Feeds.ApplyFeedUpdates(ctx, details, feed); err != nil {
		return fmt.Errorf("apply feed updates: %w", err)
	}

	slogctx.Info(ctx, "Finished update feed job.",
		slog.Duration("took", time.Since(start)))

	return nil
}
