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
	"sync"
	"time"

	feeds "github.com/immanent-tech/go-syndication"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
)

type UpdateFeedJobData struct {
	// FeedID is the unique ID of a feed.
	FeedID  models.FeedID `json:"feed_id" validate:"required,startswith=feed_"`
	URLs    []models.URL  `json:"URLs"    validate:"required,dive,url"`
	Deleted bool          `json:"deleted"`
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
	jobData, ok := updateFeedBufPool.Get().(*UpdateFeedJobData)
	defer updateFeedBufPool.Put(jobData)
	if !ok {
		return errors.New("unable to allocate job data buffer")
	}
	if err := json.Unmarshal(job.JobData, jobData); err != nil {
		return fmt.Errorf("unmarshal job data: %w", err)
	}

	// Add feed id as slog attribute for log tracking.
	ctx = slogctx.With(ctx, "feed_id", jobData.FeedID)

	jobCtx, cancel := context.WithTimeout(ctx, defaultJobTimeout)
	defer cancel()

	// Retrieve the feed details.
	details, err := models.GetFeedByID(ctx, jobData.FeedID)
	if err != nil {
		// If the returned error indicates there is no feed with the given ID, mark the job to be deleted.
		if errors.Is(err, elastic.ErrNotFound) {
			if err := models.UpdateFeed(ctx, jobData.FeedID, map[string]any{"job_data.deleted": true}); err != nil {
				return fmt.Errorf("mark job for deletion: %w", err)
			}
		}
		return fmt.Errorf("get feed doc: %w", err)
	}

	// Add additional feed details to logs.
	ctx = slogctx.With(ctx, "feed_url", jobData.URLs[0])
	ctx = slogctx.With(ctx, "feed_name", details.GetTitle())

	// Get new items since the last fetch.
	feed, err := feeds.NewFeedFromURL(jobCtx, jobData.URLs[0])
	if err != nil {
		return fmt.Errorf("fetch feed: %w", err)
	}
	items := make(models.Items, 0, len(feed.GetItems()))
	for i := range slices.Values(feed.GetItems()) {
		items = append(items, models.NewFeedItem(&i, details))
	}
	slogctx.FromCtx(ctx).Debug("Checking for new items.",
		slog.Time("since", details.LastFetched),
		slog.Int("total_items", len(items)),
		slog.Duration("interval", time.Duration(details.UpdateInterval)),
	)
	// Add any new items since the last feed update.
	if newItems := items.FilterSince(details.LastFetched); len(newItems) > 0 {
		// Add any new items.
		if err := models.AddItems(ctx, newItems...); err != nil {
			return fmt.Errorf("add new items: %w", err)
		}
		slogctx.FromCtx(ctx).Debug("Added new items.",
			slog.Int("count", len(newItems)),
		)
		// Update the last fetched field of the feed to the latest article timestamp. This will ensure we always fetch
		// newer articles where a feed lags behind real-time.
		if err := elastic.UpdateDoc(ctx, models.FeedsIndexRW, jobData.FeedID, map[string]any{
			"last_fetched": items.SortByTimestamp()[0].GetTimestamp(),
		}); err != nil {
			return fmt.Errorf("update feed last fetched: %w", err)
		}
	}
	return nil
}

var updateFeedBufPool = sync.Pool{
	New: func() any {
		return &UpdateFeedJobData{}
	},
}
