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
	"sync"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/foragd/config"
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
	feed, feedURL, err := fetchNewFeedData(ctx, data, details)
	if err != nil {
		return err
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
		// Try to add images to any items missing an image.
		for item := range slices.Values(newItems) {
			if item.GetImage() == nil {
				if imgURL, err := service.ExtractMainImage(ctx, item.GetLink()); err == nil && imgURL != "" {
					item.Image = models.NewRemoteImage(imgURL, item.GetTitle())
				}
			}
		}

		// Add new items.
		results, err := addItems(ctx, newItems)
		if err != nil {
			return fmt.Errorf("add new items: %w", err)
		}
		if len(results) > 0 {
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

			if err := updateFeed(
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
		// Update FeedStatus.
		logMsg.StatusCode = http.StatusOK
	} else {
		// Update FeedStatus.
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

func fetchNewFeedData(ctx context.Context, data UpdateFeedJob, details *models.Feed) (*models.Feed, models.URL, error) {
	// Set fetch options.
	var (
		proxyRequest bool
	)
	if details.FetchMethod == models.FeedFetchMethodProxied {
		proxyRequest = true
	}

	// Get new items since the last fetch. Try each listed source URL for the feed until one succeeds.
	var (
		feed    *models.Feed
		feedURL models.URL
	)
	for feedURL = range slices.Values(details.GetSourceURLs()) {
		var err error
		feed, err = service.FetchFeed(
			ctx,
			feedURL,
			service.FetchWithFeedID(data.FeedID),
			service.FetchWithProxy(proxyRequest),
		)
		if err != nil {
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
			continue
		}
		return feed, feedURL, nil
	}

	// No feed data returned by any url. Log and return error.
	logMsg := newFeedStatusMsg(details.GetID())
	logMsg.StatusMessage = new("no feed data returned by any URL")
	logMsg.StatusCode = http.StatusNotFound
	if err := logMsg.log(ctx); err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to record feed status.",
			slog.Any("error", err),
		)
	}

	return nil, "", fmt.Errorf(
		"%w: %s (%s)",
		ErrFetchFailed,
		*logMsg.StatusMessage,
		strings.Join(details.GetSourceURLs(), ","),
	)
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
			return nil, fmt.Errorf("bulk add items: %w", err)
		}
		return map[string]models.Items{"updated": items}, nil
	}

	// Collect updated items. Ignore noop updates like timestamp changes.
	// TODO: add a custom comparer when ExtensionData contains information worth updating.
	updatedItems := make(models.Items, 0, len(existingItems))
	for existingItem := range slices.Values(existingItems) {
		if newItem := items.FindByID(existingItem.GetID()); newItem != nil {
			if diff := cmp.Diff(*existingItem, *newItem,
				cmpopts.IgnoreFields(models.Item{}, "Updated", "Published", "Timestamp", "ExtensionData"),
				cmpopts.EquateEmpty(),
				cmpopts.IgnoreUnexported(),
			); diff != "" {
				updatedItems = append(updatedItems, newItem)
			}
		}
	}

	// Collect new items.
	newItems := items.ExcludeIDs(existingItems.GetIDs()...)

	wg, jobCtx := errgroup.WithContext(ctx)
	defer jobCtx.Done()
	results := make(map[string]models.Items)
	var mu sync.Mutex

	// Update updated items.
	if len(updatedItems) > 0 {
		wg.Go(func() error {
			if err := bulk.IndexDocuments(ctx, schema.ItemsIndexRW(), updatedItems...); err != nil {
				return fmt.Errorf("bulk update items: %w", err)
			}
			mu.Lock()
			defer mu.Unlock()
			results["updated"] = updatedItems
			return nil
		})
	}

	// Add new items.
	if len(newItems) > 0 {
		wg.Go(func() error {
			if err := bulk.IndexDocuments(ctx, schema.ItemsIndexRW(), newItems...); err != nil {
				return fmt.Errorf("bulk add items: %w", err)
			}
			mu.Lock()
			defer mu.Unlock()
			results["new"] = newItems
			return nil
		})
	}

	if err := wg.Wait(); err != nil {
		return nil, fmt.Errorf("add/update items: %w", err)
	}

	return results, nil
}

func updateFeed(ctx context.Context, oldData, newData *models.Feed, lastFetched time.Time) error {
	// If the feed does not have categories, use the classifier to generate some.
	if len(newData.GetCategories()) == 0 {
		newData.Categories = service.ClassifyFeed(ctx, newData)
	}
	// Compare new/old feed data and update as appropriate.
	if diff := cmp.Diff(*oldData, *newData,
		cmpopts.IgnoreFields(models.Feed{}, "Updated", "Published", "LastFetched", "CreatedAt"),
		cmpopts.EquateEmpty(),
		cmpopts.IgnoreUnexported(),
	); diff != "" {
		// Update feed data.
		newData.LastFetched = lastFetched
		if err := bulk.AddAction(ctx,
			bulk.NewAction(
				newData,
				bulk.AsOperation[models.FeedID](bulk.OpIndex),
				bulk.ToIndex[models.FeedID](schema.FeedsIndexRW()),
			),
		); err != nil {
			return fmt.Errorf("update feed: %w", err)
		}
	} else {
		// No changes. Just update last_fetched.
		if err := bulk.AddAction(ctx,
			bulk.NewAction(&bulk.PartialDocument{
				Parts: map[string]any{
					"last_fetched": lastFetched,
				},
				ID: newData.GetID(),
			},
				bulk.AsOperation[string](bulk.OpUpdate),
				bulk.ToIndex[string](schema.FeedsIndexRW()),
			),
		); err != nil {
			return fmt.Errorf("update feed last_fetched: %w", err)
		}
	}

	return nil
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
