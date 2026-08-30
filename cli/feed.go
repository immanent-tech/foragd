// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/validation"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/zyte"
	"github.com/immanent-tech/foragd/scheduler"
	"github.com/immanent-tech/foragd/scheduler/jobs"
	"github.com/immanent-tech/foragd/service"
)

// FeedCmd contains subcommands for interacting with feeds.
type FeedCmd struct {
	Fetch        FetchFeedCmd        `cmd:"" help:"fetch a feed (by either URL or ID)"`
	ResetUpdates ResetFeedUpdatesCmd `cmd:"" help:"reset the feed updates job"`
	Update       UpdateFeedCmd       `cmd:"" help:"update the feed"`
	AddCustom    AddCustomFeedCmd    `cmd:"" help:"add a custom feed"`
}

// FetchFeedCmd is a command that will fetch a feed, by either URL or its Feed ID.
type FetchFeedCmd struct {
	FeedID   *models.FeedID `help:"ID of feed"        validate:"omitempty,required_without=FeedURL,startswith=feed_"`
	FeedURL  *string        `help:"URL of feed"       validate:"omitempty,required_without=FeedID,omitempty,url"`
	Validate bool           `help:"validate the feed"                                                                default:"false"`
}

// Run performs the operations for fetching feed details.
func (c *FetchFeedCmd) Run() error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	if err := validation.Validate.Struct(c); err != nil {
		return fmt.Errorf("validate options: %w", err)
	}

	var (
		details *models.Feed
		feed    *models.Feed
		err     error
	)

	switch {
	case c.FeedID != nil:
		details, err = service.GetFeed(ctx, *c.FeedID)
		if err != nil {
			return fmt.Errorf("get existing feed details: %w", err)
		}
		switch details.FetchMethod {
		case models.FeedFetchMethodZyteArticles:
			feed, _, err = service.FetchFeedUpdatesAsArticles(ctx, details)
		case models.FeedFetchMethodDirect, models.FeedFetchMethodProxied:
			fallthrough
		default:
			feed, _, err = service.FetchFeedUpdates(ctx, details)
		}
	case c.FeedURL != nil:
		var feedURL *url.URL
		feedURL, err = service.NormalizeFeedURL(*c.FeedURL)
		if err != nil {
			return fmt.Errorf("parse url: %w", err)
		}
		feed, err = service.FetchFeed(ctx, feedURL.String())
	default:
		return errors.New("no fetch method specified")
	}
	if err != nil {
		return fmt.Errorf("fetch feed: %w", err)
	}

	if details != nil {
		if newItems := feed.GetItems().FilterSince(details.LastFetched); len(newItems) > 0 {
			slogctx.FromCtx(ctx).Info("Feed has new items.",
				slog.Int("count", len(newItems)),
			)
			// Try to enrich item with additional data if possible.
			var wg sync.WaitGroup
			for item := range slices.Values(newItems) {
				wg.Go(func() {
					if err := service.EnrichItem(ctx, feed, item); err != nil {
						slogctx.FromCtx(ctx).Warn("Unable to enrich item.",
							slog.Any("error", err),
						)
					}
				})
			}
			wg.Wait()
		}
	}

	showFeedDetails(feed)

	return nil
}

// ResetFeedUpdatesCmd is a CLI command to reset the updates for a feed. It will reset the last_fetched timestamp on the
// feed and delete any scheduled job for feed updates.
type ResetFeedUpdatesCmd struct {
	FeedID models.FeedID `help:"ID of feed" validate:"required,startswith=feed_"`
}

// Run performs the operations for resetting feed updates.
func (c *ResetFeedUpdatesCmd) Run() error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	if err := validation.Validate.Struct(c); err != nil {
		return fmt.Errorf("validate options: %w", err)
	}

	// Reset the last_fetched timestamp on the feed.
	if err := service.UpdateFeed(ctx, c.FeedID, map[string]any{
		"last_fetched": models.UnixEpoch,
	}); err != nil {
		return fmt.Errorf("reset feed last_fetched: %w", err)
	}
	slogctx.FromCtx(ctx).Info("Feed last_fetched reset.")

	// Delete scheduled job for feed.
	if err := scheduler.NewManager(ctx); err != nil {
		return fmt.Errorf("could not run scheduler: %w", err)
	}
	if err := scheduler.Manager.DeleteJob(
		quartz.NewJobKeyWithGroup(c.FeedID, string(jobs.JobTypeUpdateFeed)),
	); err != nil {
		return fmt.Errorf("delete feed job: %w", err)
	}
	slogctx.FromCtx(ctx).Info("Deleted existing feed job.")

	return nil
}

// UpdateFeedCmd performs updates on a feed.
type UpdateFeedCmd struct {
	FeedID                models.FeedID `help:"ID of feed"                                   validate:"required,startswith=feed_"`
	Interval              string        `help:"update the interval to the given value"`
	FetchItemDescriptions bool          `help:"always fetch item descriptions when updating"`
}

// Run performs the operations for resetting feed updates.
func (c *UpdateFeedCmd) Run() error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	if err := validation.Validate.Struct(c); err != nil {
		return fmt.Errorf("validate options: %w", err)
	}

	feed, err := service.GetFeed(ctx, c.FeedID)
	if err != nil {
		return fmt.Errorf("get feed: %w", err)
	}

	// Process required updates.
	updates := make(map[string]any)
	switch {
	case c.Interval != "":
		interval, err := time.ParseDuration(c.Interval)
		if err != nil {
			return fmt.Errorf("parse interval: %w", err)
		}
		updates["update_interval"] = interval
		// Update the feed.
		if err := service.UpdateFeed(ctx, c.FeedID, updates); err != nil {
			return fmt.Errorf("update feed: %w", err)
		}
		// Delete scheduled job for feed.
		if err := scheduler.NewManager(ctx); err != nil {
			return fmt.Errorf("could not run scheduler: %w", err)
		}
		if err := scheduler.Manager.DeleteJob(
			quartz.NewJobKeyWithGroup(c.FeedID, string(jobs.JobTypeUpdateFeed)),
		); err != nil {
			return fmt.Errorf("delete feed job: %w", err)
		}
		// Create a new job for the feed.
		newJob, err := jobs.NewUpdateFeedJob(ctx, c.FeedID)
		if err != nil {
			return fmt.Errorf("create new feed job: %w", err)
		}
		// Schedule the new job.
		if err = scheduler.Manager.ScheduleJob(newJob.JobDetail(), newJob.Trigger()); err != nil {
			return fmt.Errorf("schedule feed job: %w", err)
		}
		slogctx.FromCtx(ctx).Debug("Added new job for feed.",
			slog.String("job_id", newJob.JobDetail().JobKey().String()),
			slog.String("job_schedule", newJob.Trigger().Description()),
		)
	case c.FetchItemDescriptions:
		var quirks *models.FeedQuirks
		if feed.Quirks == nil {
			quirks = &models.FeedQuirks{}
		} else {
			quirks = feed.Quirks
		}
		// quirks.FetchItemSummaries = true
		updates["quirks"] = quirks
		// Update the feed.
		if err := service.UpdateFeed(ctx, c.FeedID, updates); err != nil {
			return fmt.Errorf("update feed: %w", err)
		}
	}

	slogctx.FromCtx(ctx).Info("Feed updated.",
		slog.String("feed_id", c.FeedID))

	return nil
}

// AddCustomFeedCmd is a command that will add a custom feed with the given options.
type AddCustomFeedCmd struct {
	FeedURL        string  `help:"URL of feed"                                 validate:"required,url"`
	Name           *string `help:"Optional name of the feed"`
	UpdateInterval string  `help:"update interval for feed"                    validate:"required"`
	WithBrowser    bool    `help:"use a browser request for fetching the feed"`
}

func (c *AddCustomFeedCmd) Run() error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	if err := validation.Validate.Struct(c); err != nil {
		return fmt.Errorf("validate options: %w", err)
	}

	// Parse the given URL.
	feedURL, err := url.Parse(c.FeedURL)
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}

	// Parse the given update interval.
	updateInterval, err := time.ParseDuration(c.UpdateInterval)
	if err != nil {
		return fmt.Errorf("parse update interval: %w", err)
	}

	// Parse the extraction options.
	extractFrom := zyte.ExtractFromHttpResponseBody
	if c.WithBrowser {
		extractFrom = zyte.ExtractFromBrowserHtml
	}

	// Fetch the details with Zyte.
	resp, err := zyte.Proxy(
		ctx,
		feedURL.String(),
		zyte.WithExtractFrom(extractFrom),
		zyte.AsArticleList(&zyte.ExtractOptions{ExtractFrom: &extractFrom}),
		zyte.WithTag("action", "new_custom_feed"),
	)
	if err != nil {
		return fmt.Errorf("fetch details: %w", err)
	}

	// Generate the feed from the Zyte response.
	feed, err := service.NewFeedFromZyteResponse(ctx, resp)
	if err != nil {
		return fmt.Errorf("generate feed: %w", err)
	}
	feed.UpdateInterval = int64(updateInterval)

	// Add any optionally specified values.
	if c.Name != nil {
		feed.Title = *c.Name
	}

	// Add the new feed.
	if err := service.AddFeed(ctx, feed); err != nil {
		return fmt.Errorf("add feed: %w", err)
	}

	slogctx.Info(ctx, "Feed added.",
		slog.String("feed_id", feed.GetID()),
		slog.String("feed_url", feed.GetSourceURLs()[0]),
	)

	return nil
}

func showFeedDetails(feed *models.Feed) {
	var str strings.Builder

	str.WriteString("Feed: ")
	str.WriteString(feed.GetTitle())
	str.WriteRune('\n')
	str.WriteString("Link: ")
	str.WriteString(feed.GetLink())
	str.WriteRune('\n')
	str.WriteString("Type: ")
	str.WriteString(string(feed.SourceType))
	str.WriteRune('\n')
	if feed.GetDescription() != "" {
		str.WriteString("Description:")
		str.WriteRune('\n')
		str.WriteString(feed.GetDescription())
		str.WriteRune('\n')
	}
	str.WriteString("Updated: ")
	str.WriteString(feed.GetTimestamp().String())
	str.WriteRune('\n')
	if len(feed.GetCategories()) > 0 {
		str.WriteString("Categories: ")
		str.WriteString(strings.Join(feed.GetCategories(), ","))
		str.WriteRune('\n')
	}
	if feed.GetImage() != nil {
		str.WriteString("Image: ")
		str.WriteString(feed.GetImage().String())
	}
	str.WriteRune('\n')
	str.WriteRune('\n')

	for article := range slices.Values(feed.GetItems().SortByTimestamp()) {
		str.WriteString("---")
		str.WriteRune('\n')
		str.WriteString("Item ID: ")
		str.WriteString(article.GetID())
		str.WriteRune('\n')
		str.WriteString("Title: ")
		str.WriteString(article.GetTitle())
		str.WriteRune('\n')
		str.WriteString("Link: ")
		str.WriteString(article.GetLink())
		str.WriteRune('\n')
		if article.GetDescription() != "" {
			str.WriteString("Description:")
			str.WriteRune('\n')
			str.WriteString(article.GetDescription())
			str.WriteRune('\n')
		}
		str.WriteString("Published: ")
		str.WriteString(article.GetTimestamp().String())
		str.WriteRune('\n')
		if len(article.GetCategories()) > 0 {
			str.WriteString("Categories: ")
			str.WriteString(strings.Join(article.GetCategories(), ","))
			str.WriteRune('\n')
		}
		if article.GetImage() != nil {
			str.WriteString("Image: ")
			str.WriteString(article.GetImage().String())
		}
		if article.GetContent() != "" {
			str.WriteString("Content:")
			str.WriteRune('\n')
			str.WriteString(article.GetContent())
		}
		str.WriteRune('\n')
	}

	fmt.Fprintf(os.Stdout, "%s", str.String())
}
