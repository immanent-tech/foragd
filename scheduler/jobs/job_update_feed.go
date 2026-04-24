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

	slogctx "github.com/veqryn/slog-context"

	feeds "github.com/immanent-tech/go-syndication"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
)

const JobTypeUpdateFeed jobType = "update_feed"

var ErrFetchFailed = errors.New("fetching feed details failed")

type UpdateFeedJobData struct {
	// FeedID is the unique ID of a feed.
	FeedID  models.FeedID `json:"feed_id" validate:"required,startswith=feed_"`
	Deleted bool          `json:"deleted"`
}

// NewUpdateFeedJob creates a job that can be scheduled from the given feed data.
func NewUpdateFeedJob(id models.FeedID, trigger *pollTrigger) (*ScheduledJob, error) {
	job := &ScheduledJob{
		CreatedAt:      time.Now().UTC(),
		JobTriggerType: jobTriggerTypePoll,
		JobType:        JobTypeUpdateFeed,
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
	data, err = json.Marshal(UpdateFeedJobData{FeedID: id})
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
			slogctx.FromCtx(ctx).Warn("No feed found with that ID. Marking update feed job for deletion.",
				slog.String("feed_id", jobData.FeedID),
			)
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
	for feedURL = range slices.Values(details.GetSourceURLs()) {
		var err error
		feed, err = models.NewFeedFromURL(ctx, feedURL, jobData.FeedID, false)
		if err != nil {
			var httpErr feeds.ParseError
			logMsg := &feedStatusLogMsg{
				FeedStatus: &models.FeedStatus{
					Timestamp: time.Now().UTC(),
					FeedID:    details.GetID(),
					URL:       feedURL,
				},
				Labels: map[string]string{
					"env":  config.CurrentEnvironment.String(),
					"type": "feed-status",
				},
			}

			if errors.Is(err, &httpErr) {
				logMsg.StatusCode = httpErr.Code
				logMsg.StatusMessage = new(httpErr.Error())
			} else {
				logMsg.StatusCode = http.StatusInternalServerError
				logMsg.StatusMessage = new(err.Error())
			}
			if _, err := elastic.BulkAdd(ctx, "logs", logMsg); err != nil {
				slogctx.FromCtx(ctx).Warn("Unable to record feed status.",
					slog.Any("error", err),
				)
			}
			continue
		}
		break
	}

	// Record the feed URL used in the logs.
	ctx = slogctx.With(ctx, "feed_url", feedURL)

	// Create a new FeedStatus for this update.
	logMsg := &feedStatusLogMsg{
		FeedStatus: &models.FeedStatus{
			Timestamp: time.Now().UTC(),
			FeedID:    details.GetID(),
			URL:       feedURL,
		},
		Labels: map[string]string{
			"env":  config.CurrentEnvironment.String(),
			"type": "feed-status",
		},
	}

	// If no feed details were returned, fail the job.
	if feed == nil {
		if logMsg.StatusCode == 0 {
			logMsg.StatusCode = http.StatusInternalServerError
		}
		if logMsg.StatusMessage != nil {
			logMsg.StatusMessage = new("no feed data returned by any URL: " + *logMsg.StatusMessage)
		} else {
			logMsg.StatusMessage = new("no feed data returned by any URL")
		}
		if _, err := elastic.BulkAdd(ctx, "logs", logMsg); err != nil {
			slogctx.FromCtx(ctx).Warn("Unable to record feed status.",
				slog.Any("error", err),
			)
		}
		return fmt.Errorf(
			"%w: %s (%s)",
			ErrFetchFailed,
			*logMsg.StatusMessage,
			strings.Join(details.GetSourceURLs(), ","),
		)
	}

	// Add any new items since the last feed update.
	if len(feed.GetItems()) == 0 {
		slogctx.FromCtx(ctx).Warn("Feed data did not contain any items.")
		return nil
	}
	if newItems := feed.GetItems().FilterSince(details.LastFetched); len(newItems) > 0 {
		slogctx.FromCtx(ctx).Debug("Found new items.",
			slog.Time("since", details.LastFetched),
			slog.Int("total_items", len(feed.GetItems())),
			slog.Int("new_items", len(newItems)),
			slog.Duration("interval", time.Duration(details.UpdateInterval)),
		)
		// Add any new items.
		if err := models.AddItems(ctx, newItems...); err != nil {
			return fmt.Errorf("add new items: %w", err)
		}
		// Update the last fetched field of the feed to the latest article timestamp. This will ensure we always fetch
		// newer articles where a feed lags behind real-time.
		updates := generateFeedUpdates(feed, details)
		updates["last_fetched"] = newItems.SortByTimestamp()[0].GetTimestamp()
		if err := elastic.UpdateDoc(ctx, schema.FeedsIndexRW, jobData.FeedID, updates); err != nil {
			return fmt.Errorf("update feed: %w", err)
		}
		// Update FeedStatus.
		logMsg.StatusCode = http.StatusOK
		logMsg.Items = newItems.GetIDs()
	} else {
		// Update FeedStatus.
		logMsg.StatusCode = http.StatusNoContent
	}
	// Index FeedStatus for this update.
	if _, err := elastic.BulkAdd(ctx, "logs", logMsg); err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to record feed status.",
			slog.Any("error", err),
		)
	}

	return nil
}

func generateFeedUpdates(newData, oldData *models.Feed) map[string]any {
	updates := make(map[string]any)
	// Always update updated timestamp.
	updates["updated"] = newData.Updated
	// Update the feed image if it has changed.
	if oldData.GetImage() == nil {
		updates["image"] = newData.GetImage()
	} else if img := oldData.GetImage(); img.GetURL() != newData.GetImage().GetURL() {
		updates["image"] = newData.GetImage()
	}
	// Update the title if it has changed.
	if oldData.GetTitle() != newData.GetTitle() {
		updates["title"] = newData.GetTitle()
	}
	// Update the description if it has changed.
	if oldData.GetDescription() != newData.GetDescription() {
		updates["description"] = newData.GetDescription()
	}
	return updates
}

var updateFeedBufPool = sync.Pool{
	New: func() any {
		return &UpdateFeedJobData{}
	},
}

type feedStatusLogMsg struct {
	*models.FeedStatus

	Labels map[string]string `json:"labels"`
}
