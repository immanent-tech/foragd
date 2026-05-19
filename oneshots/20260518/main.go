package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"

	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/scheduler"
	"github.com/immanent-tech/foragd/scheduler/jobs"
)

func main() {
	ctx := context.TODO()

	if err := elastic.Connect(); err != nil {
		panic(err)
	}

	if err := scheduler.NewManager(ctx); err != nil {
		panic(err)
	}

	feeds, err := elastic.SearchAll[*models.Feed](ctx, schema.FeedsIndexRO(), query.MatchAll(), 5000)
	if err != nil {
		panic(err)
	}

	for feed := range slices.Values(feeds) {
		// Add additional feed details to logs.
		feedCtx := slogctx.With(ctx, "feed_id", feed.GetID())
		feedCtx = slogctx.With(feedCtx, "feed_name", feed.GetTitle())

		jobKey := quartz.NewJobKeyWithGroup(feed.GetID(), "update_feed")
		switch existingJob, err := scheduler.Manager.GetScheduledJob(jobKey); {
		case err != nil && models.HTTPStatus(err) != http.StatusNotFound && !errors.Is(err, quartz.ErrJobNotFound):
			// If we cannot ascertain if there is an existing scheduled job, skip this feed.
			slogctx.FromCtx(feedCtx).Warn("Unable to check for existing scheduled job.",
				slog.String("feed_id", feed.GetID()),
				slog.Any("error", err),
			)
		case errors.Is(err, quartz.ErrJobNotFound):
			// If there is no existing scheduled newJob, create one.
			newJob, err := jobs.NewUpdateFeedJob(ctx, feed.GetID())
			if err != nil {
				slogctx.FromCtx(feedCtx).Warn("Unable to create new update feed job for feed.",
					slog.Any("error", err),
				)
				return
			}

			// Schedule the new job.
			if err = scheduler.Manager.ScheduleJob(newJob.JobDetail(), newJob.Trigger()); err != nil {
				slogctx.FromCtx(feedCtx).Error("Failed to schedule new job for feed.",
					slog.String("job_id", newJob.JobDetail().JobKey().String()),
					slog.String("job_schedule", newJob.Trigger().Description()),
					slog.Any("error", err),
				)
				return
			}
			slogctx.FromCtx(feedCtx).Debug("Added new job for feed.",
				slog.String("job_id", newJob.JobDetail().JobKey().String()),
				slog.String("job_schedule", newJob.Trigger().Description()),
			)
			// // Do an initial run of the job.
			// if err = newJob.JobDetail().Job().Execute(ctx); err != nil {
			// 	slogctx.FromCtx(feedCtx).Error("Failed initial run of update feed job.",
			// 		slog.String("job_id", newJob.JobDetail().JobKey().String()),
			// 		slog.String("job_schedule", newJob.Trigger().Description()),
			// 		slog.Any("error", err),
			// 	)
			// }
		case existingJob != nil:
			// Existing job found, ignore.
			slogctx.FromCtx(feedCtx).Debug("Existing job found, ignoring.",
				slog.String("job_id", existingJob.JobDetail().JobKey().String()),
				slog.String("feed_id", feed.GetID()),
			)
		default:
			// Unhandled result.
			slogctx.FromCtx(feedCtx).Debug("Unhandled result.",
				slog.String("feed_id", feed.GetID()),
			)
		}

	}

}
