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
	"sync"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/config"

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

	// Create a new FeedStatus for this update.
	logMsg := newFeedStatusMsg(details.GetID())
	logMsg.FeedStatus.URL = feedURL

	// Add any new items since the last feed update.
	if len(feed.GetItems()) == 0 {
		logMsg.StatusCode = http.StatusNoContent
		slogctx.FromCtx(ctx).Warn("Feed data did not contain any items.")
		return nil
	}
	if newItems := feed.GetItems().FilterSince(details.LastFetched); len(newItems) > 0 {
		const maxConcurrentEnrichment = 25
		sem := make(chan struct{}, maxConcurrentEnrichment)
		defer close(sem)
		// Try to enrich item with additional data if possible.
		var wg sync.WaitGroup
		for item := range slices.Values(newItems) {
			sem <- struct{}{}
			wg.Go(func() {
				defer func() { <-sem }()
				if err := service.EnrichItem(ctx, feed, item); err != nil {
					slogctx.FromCtx(ctx).Warn("Unable to enrich item.",
						slog.Any("error", err),
					)
				}
			})
		}
		wg.Wait()

		// Add new items.
		results, err := addItems(ctx, newItems)
		if err != nil {
			return fmt.Errorf("add new items: %w", err)
		}
		if len(results["new"]) > 0 || len(results["updated"]) > 0 {
			slogctx.FromCtx(ctx).Debug("Added new/updated items.",
				slog.Time("since", details.LastFetched),
				slog.Int("new", len(results["new"])),
				slog.Int("updated", len(results["updated"])),
			)
			var allItems models.Items
			for _, v := range results {
				allItems = append(allItems, v...)
			}
			// Update the last fetched field of the feed to the latest article timestamp. This will ensure we always fetch
			// newer articles where a feed lags behind real-time.
			if err := service.ApplyFeedUpdates(
				ctx,
				details,
				feed,
				allItems.SortByTimestamp()[0].GetTimestamp(),
			); err != nil {
				slogctx.FromCtx(ctx).Warn("Unable to update feed details.",
					slog.Any("error", err),
				)
			}
			logMsg.Items = allItems.GetIDs()
		}
		logMsg.StatusCode = http.StatusOK
	} else {
		logMsg.StatusCode = http.StatusNoContent
	}
	// Index FeedStatus for this update.
	if err := logMsg.log(ctx); err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to record feed status.",
			slog.Any("error", err),
		)
	}

	if err := bulk.Flush(ctx); err != nil {
		return fmt.Errorf("update feed job: flush updates: %w", err)
	}

	slogctx.FromCtx(ctx).Debug("Finished update feed job.",
		slog.Duration("took", time.Since(start)))

	return nil
}

func addItems(ctx context.Context, items models.Items) (map[string]models.Items, error) {
	// Get any existing versions of the items.
	existingItems, err := service.GetItems(ctx, items.GetIDs()...)
	if err != nil {
		slogctx.FromCtx(ctx).
			Warn("Could not fetch existing items for comparing updates, falling back to bulk update of all items.",
				slog.Any("error", err),
			)
		if err := bulk.IndexDocuments(ctx, schema.ItemsIndexRW(), items...); err != nil {
			return nil, models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("bulk add items: %w", err))
		}
		return map[string]models.Items{"updated": items}, nil
	}

	// Collect updated items. Ignore no-op updates like timestamp changes.
	// TODO: add a custom comparer when ExtensionData contains information worth updating.
	updatedItems := make(models.Items, 0, len(existingItems))
	for existingItem := range slices.Values(existingItems) {
		if updatedItem := items.FindByID(existingItem.GetID()); updatedItem != nil {
			if diff := cmp.Diff(
				*existingItem,
				*updatedItem,
				cmpopts.IgnoreFields(
					models.Item{},
					"Updated",
					"Published",
					"Timestamp",
					"ExtensionData",
					"ExtensionType",
				),
				cmpopts.EquateEmpty(),
				cmpopts.IgnoreUnexported(),
			); diff != "" {
				updatedItems = append(updatedItems, updatedItem)
			}
		}
	}

	// Collect new items.
	newItems := items.ExcludeIDs(existingItems.GetIDs()...)

	// Index all items.
	results := make(map[string]models.Items)
	results["updated"] = updatedItems
	results["new"] = newItems
	if err := bulk.IndexDocuments(ctx, schema.ItemsIndexRW(), slices.Concat(updatedItems, newItems)...); err != nil {
		return nil, models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("bulk add/update items: %w", err))
	}

	return results, nil
}

type feedStatusLogMsg struct {
	*models.FeedStatus

	Labels map[string]string `json:"labels"`
}

func newFeedStatusMsg(id models.FeedID) *feedStatusLogMsg {
	return &feedStatusLogMsg{
		FeedStatus: &models.FeedStatus{
			Timestamp: time.Now().UTC(),
			FeedID:    id,
		},
		Labels: map[string]string{
			"env":  config.GetEnvironment().String(),
			"type": "feed-status",
		},
	}
}

func (l *feedStatusLogMsg) log(ctx context.Context) error {
	if err := bulk.AddAction(ctx,
		bulk.NewAction(
			l,
			bulk.AsOperation[string](bulk.OpIndex),
			bulk.ToIndex[string]("logs"),
		),
	); err != nil {
		return fmt.Errorf("add bulk action: %w", err)
	}
	return nil
}

func logZyteError(ctx context.Context, err error, feedURL string, details *models.Feed) {
	logMsg := newFeedStatusMsg(details.GetID())
	logMsg.FeedStatus.URL = feedURL
	if apiErr, ok := errors.AsType[*models.APIError](err); ok {
		logMsg.StatusCode = apiErr.StatusCode
		logMsg.StatusMessage = new(apiErr.Error())
	} else {
		logMsg.StatusCode = http.StatusInternalServerError
		logMsg.StatusMessage = new(err.Error())
	}
	if err := logMsg.log(ctx); err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to record feed status.",
			slog.Any("error", err),
		)
	}
}

func logGeneralError(ctx context.Context, err error, feedURL string, details *models.Feed) {
	logMsg := newFeedStatusMsg(details.GetID())
	logMsg.FeedStatus.URL = feedURL
	if apiErr, ok := errors.AsType[*models.APIError](err); ok {
		logMsg.StatusCode = apiErr.StatusCode
		logMsg.StatusMessage = new(apiErr.Error())
	} else {
		logMsg.StatusCode = http.StatusInternalServerError
		logMsg.StatusMessage = new(err.Error())
	}
	if err := logMsg.log(ctx); err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to record feed status.",
			slog.Any("error", err),
		)
	}
}
