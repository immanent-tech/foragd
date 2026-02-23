// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	feeds "github.com/immanent-tech/go-syndication"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
)

const jobTypeUpdateFeed jobType = "update_feed"

var ErrFetchFailed = errors.New("fetching feed details failed")

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

	ctx, cancel := context.WithTimeout(ctx, defaultJobTimeout)
	defer cancel()

	// Add feed id as slog attribute for log tracking.
	ctx = slogctx.With(ctx, "feed_id", jobData.FeedID)

	// Retrieve the feed details.
	details, err := models.GetFeedByID(ctx, jobData.FeedID)
	if err != nil {
		// If the returned error indicates there is no feed with the given ID, mark the job to be deleted.
		if errors.Is(err, elastic.ErrNotFound) {
			if err := models.UpdateFeed(ctx, jobData.FeedID, map[string]any{"job_data.deleted": true}); err != nil {
				return fmt.Errorf("mark job for deletion: %w", err)
			}
		} else {
			return fmt.Errorf("get feed doc: %w", err)
		}
	}

	// Add additional feed details to logs.
	ctx = slogctx.With(ctx, "feed_name", details.GetTitle())

	// Get new items since the last fetch. Try each listed source URL for the feed until one succeeds.
	var (
		feed    *models.Feed
		feedURL models.URL
	)
	for feedURL = range slices.Values(jobData.URLs) {
		var err error
		feed, err = models.NewFeedFromURL(ctx, feedURL, jobData.FeedID, true)
		if err != nil {
			var httpErr feeds.HTTPError
			status := &models.FeedStatus{
				Timestamp: time.Now().UTC(),
				FeedID:    details.GetID(),
				URL:       feedURL,
			}
			if errors.Is(err, &httpErr) {
				status.StatusCode = httpErr.Code
				status.StatusMessage = &httpErr.Message
			} else {
				status.StatusCode = http.StatusInternalServerError
				status.StatusMessage = new(err.Error())
			}
			if err := models.AddFeedStatus(ctx, status); err != nil {
				slogctx.FromCtx(ctx).Warn("Unable to record feed status.",
					slog.Any("error", err),
				)
			}
			continue
		}
		break
	}

	if feed == nil {
		return fmt.Errorf("%w: all source URLs returned errors", ErrFetchFailed)
	}

	ctx = slogctx.With(ctx, "feed_url", feedURL)

	// Create a new FeedStatus for this update.
	status := &models.FeedStatus{
		Timestamp: time.Now().UTC(),
		FeedID:    details.GetID(),
	}

	// Add any new items since the last feed update.
	if len(feed.GetItems()) > 0 {
		slogctx.FromCtx(ctx).Debug("Checking for new items.",
			slog.Time("since", details.LastFetched),
			slog.Int("total_items", len(feed.GetItems())),
			slog.Duration("interval", time.Duration(details.UpdateInterval)),
		)
		if newItems := feed.GetItems().FilterSince(details.LastFetched); len(newItems) > 0 {
			// Add any new items.
			if err := models.AddItems(ctx, newItems...); err != nil {
				return fmt.Errorf("add new items: %w", err)
			}
			slogctx.FromCtx(ctx).Debug("Added new items.",
				slog.Int("count", len(newItems)),
			)
			// Update the last fetched field of the feed to the latest article timestamp. This will ensure we always fetch
			// newer articles where a feed lags behind real-time.
			if err := elastic.UpdateDoc(ctx, schema.FeedsIndexRW, jobData.FeedID, map[string]any{
				"last_fetched": newItems.SortByTimestamp()[0].GetTimestamp(),
			}); err != nil {
				return fmt.Errorf("update feed last fetched: %w", err)
			}
			// Update FeedStatus.

			status.StatusCode = http.StatusOK
			status.StatusMessage = new(
				fmt.Sprintf("added %d new items: %s", len(newItems), strings.Join(newItems.GetIDs(), ",")),
			)
		} else {
			// Update FeedStatus.
			status.StatusCode = http.StatusNoContent
			status.StatusMessage = new("no new items")
		}
		// Index FeedStatus for this update.
		if err := models.AddFeedStatus(ctx, status); err != nil {
			slogctx.FromCtx(ctx).Warn("Unable to record feed status.",
				slog.Any("error", err),
			)
		}
	}

	return nil
}

var updateFeedBufPool = sync.Pool{
	New: func() any {
		return &UpdateFeedJobData{}
	},
}
