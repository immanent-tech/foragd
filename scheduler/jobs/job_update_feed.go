// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	feeds "github.com/immanent-tech/go-syndication"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
)

type UpdateFeedJobData struct {
	// FeedID is the unique ID of a feed.
	FeedID models.FeedID `form:"feed_id" json:"feed_id" validate:"required,startswith=feed_"`
	URLs   []models.URL  `               json:"URLs"    validate:"required,dive,url"`
}

// NewUpdateFeedJob creates a job that can be scheduled from the given feed data.
func NewUpdateFeedJob(id models.FeedID, urls []models.URL, trigger *pollTrigger) (*ScheduledJob, error) {
	job := &ScheduledJob{
		CreatedAt:      time.Now().UTC(),
		JobTriggerType: jobTriggerTypePoll,
		JobType:        jobTypeUpdateFeed,
		JobDescription: "Get new items for " + id,
	}

	var (
		data []byte
		err  error
	)

	// Create trigger.
	data, err = json.Marshal(trigger)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateJobFailed, err)
	}
	job.JobTrigger = data

	// Create job data.
	data, err = json.Marshal(UpdateFeedJobData{FeedID: id, URLs: urls})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateJobFailed, err)
	}
	job.JobData = data

	return job, nil
}

// executeUpdateFeedJob will execute a job that attempts to find new items for a feed and index them into the data
// backend.
func executeUpdateFeedJob(ctx context.Context, job *ScheduledJob) error {
	var jobData UpdateFeedJobData
	if err := json.Unmarshal(job.JobData, &jobData); err != nil {
		return fmt.Errorf("%w: %w", ErrExecuteJobFailed, err)
	}

	dataAPI, ok := ctx.Value(dataAPICtxKey).(DataAPI)
	if !ok {
		return fmt.Errorf("%w: unable to get data api from context", ErrExecuteJobFailed)
	}

	// Add feed id as slog attribute for log tracking.
	ctx = slogctx.With(ctx, "feed_id", jobData.FeedID)

	jobCtx, cancel := context.WithTimeout(ctx, defaultJobTimeout)
	defer cancel()

	// Retrieve the feed details.
	details, err := dataAPI.GetFeed(jobCtx, jobData.FeedID)
	if err != nil && !errors.Is(err, ErrNoJob) {
		return fmt.Errorf("%w: %s: %w", ErrExecuteJobFailed, job.Description(), err)
	}
	// Get new items since the last fetch.
	feed, err := feeds.NewFeedFromURL(jobCtx, jobData.URLs[0])
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrExecuteJobFailed, job.Description(), err)
	}
	items := make(models.Items, 0)
	for i := range slices.Values(feed.GetItems()) {
		items = append(items, models.NewItemFromSource(&i, details))
	}
	slogctx.FromCtx(ctx).Debug("Checking for new items.",
		slog.String("feed", details.GetTitle()),
		slog.Time("since", details.LastFetched),
		slog.Int("total_items", len(items)),
	)
	// Add any new items since the last feed update.
	if len(items.FilterSince(details.LastFetched)) > 0 {
		// Add any new items.
		_, err = dataAPI.AddItems(jobCtx, items.FilterSince(details.LastFetched)...)
		if err != nil {
			return fmt.Errorf("%w: %s: %w", ErrExecuteJobFailed, job.Description(), err)
		}
		slogctx.FromCtx(ctx).Debug("Added new items.",
			slog.String("feed", details.GetTitle()),
			slog.Int("count", len(items.FilterSince(details.LastFetched))),
		)
		// Update the last fetched field of the feed to the latest article timestamp. This will ensure we always fetch
		// newer articles where a feed lags behind real-time.
		err = dataAPI.UpdateFeedLastFetched(jobCtx, jobData.FeedID, items.SortByTimestamp()[0].GetTimestamp())
		if err != nil {
			return fmt.Errorf("%w: %s: %w", ErrExecuteJobFailed, job.Description(), err)
		}
	}
	return nil
}
