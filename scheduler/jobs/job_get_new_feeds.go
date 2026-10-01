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
	"slices"
	"sync/atomic"
	"time"

	"github.com/reugn/go-quartz/matcher"
	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/foragd/models"
)

const (
	// addFeedTimeout is the maximum time to spend trying to add a feed.
	addFeedTimeout = 10 * time.Minute
)

var getNewFeedsRunning atomic.Bool

// NewGetNewFeedsJob creates a job for checking for new feeds.
func NewGetNewFeedsJob() (*SerializedJob, error) {
	job := &SerializedJob{
		CreatedAt:      time.Now().UTC(),
		JobDescription: new("Find and schedule jobs to update feeds."),
		JobKey:         quartz.NewJobKey(string(JobTypeGetNewFeeds)).String(),
		JobType:        JobTypeGetNewFeeds,
		JobNextRun:     models.UnixEpoch,
		JobTriggerType: TriggerTypePoll,
	}

	if err := job.JobData.FromGetNewFeedsJob(GetNewFeedsJob{Checkpoint: models.UnixEpoch}); err != nil {
		return nil, fmt.Errorf("create job data: %w", err)
	}

	if err := job.JobTrigger.FromPollTrigger(*NewPollTrigger(DefaultPollInterval, DefaultPollJitter)); err != nil {
		return nil, fmt.Errorf("create trigger: %w", err)
	}

	return job, nil
}

// ExecuteGetNewFeeds runs a job that will look for newly added feeds and schedule new jobs to fetch item updates for
// them.
func ExecuteGetNewFeeds(ctx context.Context, job *SerializedJob) error {
	if wasRunning := getNewFeedsRunning.Swap(true); wasRunning {
		return errors.New("job is running")
	}
	defer getNewFeedsRunning.Store(false)

	services := ServicesFromCtx(ctx)
	if services == nil {
		return errors.New("no services in context")
	}

	log := slogctx.FromCtx(ctx)

	start := time.Now()

	// Find new feeds.
	newFeeds, err := services.Feeds.GetNewFeedsSince(ctx, models.UnixEpoch)
	if err != nil {
		return fmt.Errorf("get new feeds since: %w", err)
	}
	if len(newFeeds) > 0 {
		log.Info("Found new feeds to process.",
			slog.Int("count", len(newFeeds)),
		)
	}

	// Get job keys for existing feed jobs.
	existingJobKeys, err := services.Scheduler.GetJobKeys(matcher.JobGroupEquals(string(JobTypeUpdateFeed)))
	if err != nil {
		return fmt.Errorf("get job keys: %w", err)
	}
	existing := make(map[string]struct{}, len(existingJobKeys))
	for _, k := range existingJobKeys {
		existing[k.Name()] = struct{}{}
	}

	const maxConcurrentWorkers = 25
	var (
		added atomic.Int64
		g     errgroup.Group
	)
	g.SetLimit(maxConcurrentWorkers)

	for feed := range slices.Values(newFeeds) {
		if ctx.Err() != nil {
			log.Info("Finished get new feeds job.",
				slog.Int64("added", added.Load()),
				slog.Duration("took", time.Since(start)))
			break
		}
		if _, ok := existing[feed.GetID()]; ok {
			continue // Already scheduled, waiting on next fetch.
		}
		g.Go(func() error { // Blocks when maxConcurrentWorkers are in flight.
			if err := addFeedWithTimeout(ctx, services, feed); err != nil {
				log.Error("Unable to add feed job",
					slog.String("feed_id", feed.GetID()),
					slog.Any("error", err))
				return nil // One bad feed shouldn't stop the rest.
			}
			added.Add(1)
			return nil
		})
	}
	if err = g.Wait(); err != nil {

	}

	log.Info("Finished get new feeds job.",
		slog.Int64("added", added.Load()),
		slog.Duration("took", time.Since(start)))

	return nil
}

func addFeedWithTimeout(ctx context.Context, s *Services, feed *models.Feed) error {
	ctx, cancel := context.WithTimeoutCause(ctx, addFeedTimeout,
		errors.New("exceeded max add feed timeout"))
	defer cancel()
	ctx = slogctx.With(ctx, "feed_id", feed.GetID(), "feed_name", feed.GetTitle())
	return AddFeedJob(ctx, s.Scheduler, s.Feeds, feed)
}

func AddFeedJob(
	ctx context.Context,
	scheduler SchedulerAPI,
	feedSvc FeedsAPI,
	feed *models.Feed,
) error {
	// Create update feed job.
	job, err := NewUpdateFeedJob(ctx, feedSvc, feed.GetID())
	if err != nil {
		return fmt.Errorf("new update feed job: %w", err)
	}

	ctx = slogctx.With(ctx,
		slog.String("job_id", job.JobDetail().JobKey().String()),
		slog.String("job_schedule", job.Trigger().Description()),
	)

	log := slogctx.FromCtx(ctx)

	// Do an initial run of the job.
	log.Info("Performing initial execution of update feed job.")
	if err := safeExecute(ctx, job, ExecuteUpdateFeed); err != nil {
		log.Warn("Failed initial run of update feed job. Pausing.",
			slog.Any("error", err),
		)
		feed.LastFetched = time.Now().UTC()
		if err := feedSvc.UpdateFeed(ctx, feed); err != nil {
			log.Error("Unable to update last fetched.",
				slog.Any("error", err),
			)
		}
	}
	// Schedule the new job.
	if err = scheduler.ScheduleJob(job.JobDetail(), job.Trigger()); err != nil {
		return fmt.Errorf("schedule update feed job: %w", err)
	}
	log.Info("Added update feed job.")

	return nil
}
