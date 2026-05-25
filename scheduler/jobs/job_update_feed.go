// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	feeds "github.com/immanent-tech/go-syndication"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
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

	// Determine and set the feed update interval.
	if err := feed.SetUpdateInterval(ctx); err != nil {
		return nil, fmt.Errorf("set update interval: %w", err)
	}
	trigger := NewPollTrigger(feed.UpdateInterval, DefaultPollJitter)

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
		if updateErr := elastic.UpdateDoc(
			ctx,
			schema.SchedulerIndexRW(),
			job.JobDetail().JobKey().String(),
			job,
			elastic.WithDocAsUpsert(true),
			elastic.WithRefresh(true),
		); updateErr != nil {
			return fmt.Errorf("update job: %w", updateErr)
		}
		slogctx.FromCtx(ctx).Warn("No feed found with that ID. Marking update feed job for deletion.")
	case err != nil:
		return fmt.Errorf("get feed doc: %w", err)
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
		feed, err = service.FetchFeed(ctx, feedURL, data.FeedID, false)
		if err != nil {
			var httpErr feeds.ParseError
			logMsg := &feedStatusLogMsg{
				FeedStatus: &models.FeedStatus{
					Timestamp: time.Now().UTC(),
					FeedID:    details.GetID(),
					URL:       feedURL,
				},
				Labels: map[string]string{
					"env":  config.GetEnvironment().String(),
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
			"env":  config.GetEnvironment().String(),
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
		if err := service.AddItems(ctx, newItems...); err != nil {
			return fmt.Errorf("add new items: %w", err)
		}
		// Update the last fetched field of the feed to the latest article timestamp. This will ensure we always fetch
		// newer articles where a feed lags behind real-time.
		if err := service.UpdateFeedDetails(
			ctx,
			details,
			feed,
			newItems.SortByTimestamp()[0].GetTimestamp(),
		); err != nil {
			slogctx.FromCtx(ctx).Warn("Unable to update feed details.",
				slog.Any("error", err),
			)
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

	slogctx.FromCtx(ctx).Debug("Finished update feed job.",
		slog.Duration("took", time.Since(start)))

	return nil
}

type feedStatusLogMsg struct {
	*models.FeedStatus

	Labels map[string]string `json:"labels"`
}
