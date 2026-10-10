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
	"syscall"
	"time"

	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/validation"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	"github.com/immanent-tech/foragd/providers/zyte"
	"github.com/immanent-tech/foragd/scheduler"
	"github.com/immanent-tech/foragd/scheduler/jobs"
	"github.com/immanent-tech/foragd/service"
)

type FeedArgs struct {
	DirectFetchArgs `embed:""`
	ZyteFetchArgs   `embed:""`

	Name           *string `help:"Optional name of the feed"                   optional:""`
	Description    *string `help:"Optional description of the feed"            optional:""`
	Categories     *string `help:"Optional comma-separated list of categories" optional:""`
	UpdateInterval *string `help:"update interval for feed"                    optional:""`
}

type DirectFetchArgs struct {
	models.FetchDirectOptions
}

type ZyteFetchArgs struct {
	models.FetchZyteOptions `embed:""`
}

// FeedCmd contains subcommands for interacting with feeds.
type FeedCmd struct {
	Fetch        FetchFeedCmd        `cmd:"" help:"fetch a feed (by either URL or ID)"`
	ResetUpdates ResetFeedUpdatesCmd `cmd:"" help:"reset the feed updates job"`
	Edit         EditFeedCmd         `cmd:"" help:"edit the feed"`
	Add          AddFeedCmd          `cmd:"" help:"add a feed"`
	Classify     ClassifyFeedCmd     `cmd:"" help:"classify a feed"`
}

// FetchFeedCmd is a command that will fetch a feed, by either URL or its Feed ID.
type FetchFeedCmd struct {
	FeedID       *models.FeedID `help:"ID of feed"                        validate:"omitempty,required_without=FeedURL,startswith=feed_"`
	FeedURL      *string        `help:"URL of feed"                       validate:"omitempty,required_without=FeedID,omitempty,url"`
	ApplyUpdates bool           `help:"Whether to write updates to store" validate:"omitempty"`
	Validate     bool           `help:"validate the feed"                                                                                default:"false"`
}

// Run performs the operations for fetching feed details.
func (c *FetchFeedCmd) Run() error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	if err := validation.Validate.Struct(c); err != nil {
		return fmt.Errorf("validate options: %w", err)
	}

	feedSvc, err := service.LoadFeedService()
	if err != nil {
		return fmt.Errorf("load feed service: %w", err)
	}

	var (
		details *models.Feed
		feed    *models.Feed
	)

	switch {
	case c.FeedID != nil:
		details, err = feedSvc.GetFeed(ctx, *c.FeedID)
		if err != nil {
			return fmt.Errorf("get existing feed details: %w", err)
		}
		switch details.FetchMethod {
		case models.FeedFetchMethodZyteArticles:
			feed, _, err = feedSvc.FetchFeedUpdatesAsArticles(ctx, details)
		case models.FeedFetchMethodDirect, models.FeedFetchMethodProxied:
			fallthrough
		default:
			feed, _, err = feedSvc.FetchFeedUpdates(ctx, details)
		}
	case c.FeedURL != nil:
		var feedURL *url.URL
		feedURL, err = models.NormalizeFeedURL(*c.FeedURL)
		if err != nil {
			return fmt.Errorf("parse url: %w", err)
		}
		feed, err = feedSvc.FetchFeed(ctx, feedURL.String())
	default:
		return errors.New("no fetch method specified")
	}
	if err != nil {
		return fmt.Errorf("fetch feed: %w", err)
	}

	if details != nil && c.ApplyUpdates {
		if err := feedSvc.ApplyFeedUpdates(ctx, details, feed); err != nil {
			return fmt.Errorf("apply feed updates: %w", err)
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

	feedSvc, err := service.LoadFeedService()
	if err != nil {
		return fmt.Errorf("load feed service: %w", err)
	}

	feed, err := feedSvc.GetFeed(ctx, c.FeedID)
	if err != nil {
		return fmt.Errorf("get feed %s: %w", c.FeedID, err)
	}

	// Reset the last_fetched timestamp on the feed.
	feed.LastFetched = models.UnixEpoch
	if err := feedSvc.UpdateFeed(ctx, feed); err != nil {
		return fmt.Errorf("reset feed last_fetched: %w", err)
	}
	slogctx.FromCtx(ctx).Info("Feed last_fetched reset.")

	// Delete scheduled job for feed.
	manager, err := scheduler.New()
	if err != nil {
		return fmt.Errorf("could not run scheduler: %w", err)
	}
	if err := manager.DeleteJob(
		quartz.NewJobKeyWithGroup(c.FeedID, string(jobs.JobTypeUpdateFeed)),
	); err != nil && !errors.Is(err, quartz.ErrJobNotFound) {
		return fmt.Errorf("delete feed job: %w", err)
	}
	slogctx.FromCtx(ctx).Info("Deleted existing feed job.")

	return nil
}

// EditFeedCmd performs updates on a feed.
type EditFeedCmd struct {
	FeedArgs `embed:""`

	FeedID models.FeedID `help:"ID of feed" required:"" validate:"required,startswith=feed_"`
}

// Run performs the operations for resetting feed updates.
func (c *EditFeedCmd) Run() error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	if err := validation.Validate.Struct(c); err != nil {
		return fmt.Errorf("validate options: %w", err)
	}

	feedSvc, err := service.LoadFeedService()
	if err != nil {
		return fmt.Errorf("load feed service: %w", err)
	}

	feed, err := feedSvc.GetFeed(ctx, c.FeedID)
	if err != nil {
		return fmt.Errorf("get feed: %w", err)
	}

	// Apply any update interval change
	if err := editUpateInterval(c.FeedArgs, feed); err != nil {
		return fmt.Errorf("edit update interval: %w", err)
	}

	// Apply any display changes.
	if err := editDisplay(c.FeedArgs, feed); err != nil {
		return fmt.Errorf("edit display: %w", err)
	}

	// Apply any fetch method change.
	if err := editFetchMethod(c.FeedArgs, feed); err != nil {
		return fmt.Errorf("edit fetch method: %w", err)
	}

	// Update feed.
	if err := updateFeed(ctx, c.FeedArgs, feedSvc, feed); err != nil {
		return fmt.Errorf("update feed: %w", err)
	}

	return nil
}

func editUpateInterval(edits FeedArgs, feed *models.Feed) error {
	// Edit update interval.
	if interval := edits.UpdateInterval; interval != nil {
		interval, err := time.ParseDuration(*interval)
		if err != nil {
			return fmt.Errorf("parse interval: %w", err)
		}
		feed.UpdateInterval = int64(interval)
	}
	return nil
}

func editDisplay(edits FeedArgs, feed *models.Feed) error {
	// Edit name/description/categories.
	if edits.Name != nil {
		if feed.Customisation == nil {
			feed.Customisation = &models.FeedCustomisation{}
		}
		feed.Customisation.Title = edits.Name
	}
	if edits.Description != nil {
		if feed.Customisation == nil {
			feed.Customisation = &models.FeedCustomisation{}
		}
		feed.Customisation.Description = edits.Description
	}
	if edits.Categories != nil {
		categories := strings.Split(*edits.Categories, ",")
		feed.Categories = categories
	}
	return nil
}

func editFetchMethod(edits FeedArgs, feed *models.Feed) error {
	// Edit fetch method.
	switch feed.FetchMethod {
	case models.FeedFetchMethodDirect, models.FeedFetchMethodProxied:
		if feed.FetchOptions != nil {
			// Compare and update fetch options if needed.
			currentFetchOptions, err := feed.FetchOptions.AsFetchDirectOptions()
			if err != nil {
				return fmt.Errorf("extract fetch options: %w", err)
			}
			if edits.DirectFetchArgs.FetchItemSummaries != currentFetchOptions.FetchItemSummaries {
				newFetchOptions := models.Feed_FetchOptions{}
				if err := newFetchOptions.FromFetchDirectOptions(edits.DirectFetchArgs.FetchDirectOptions); err != nil {
					return fmt.Errorf("update direct fetch options: %w", err)
				}
				feed.FetchOptions = &newFetchOptions
			}
		} else {
			// Add new fetch options.
			newFetchOptions := models.Feed_FetchOptions{}
			if err := newFetchOptions.FromFetchDirectOptions(edits.DirectFetchArgs.FetchDirectOptions); err != nil {
				return fmt.Errorf("update direct fetch options: %w", err)
			}
			feed.FetchOptions = &newFetchOptions
		}
	case models.FeedFetchMethodZyteArticles:
		currentFetchOptions, err := feed.FetchOptions.AsFetchZyteOptions()
		if err != nil {
			return fmt.Errorf("extract fetch options: %w", err)
		}
		var changed bool
		if currentFetchOptions.ExtractFrom != nil && edits.ZyteFetchArgs.ExtractFrom != nil {
			if *currentFetchOptions.ExtractFrom != *edits.ZyteFetchArgs.ExtractFrom {
				currentFetchOptions.ExtractFrom = edits.ZyteFetchArgs.ExtractFrom
				changed = true
			}
		} else if edits.ZyteFetchArgs.ExtractFrom != nil {
			currentFetchOptions = edits.ZyteFetchArgs.FetchZyteOptions
			changed = true
		}
		if changed {
			newFetchOptions := models.Feed_FetchOptions{}
			if err := newFetchOptions.FromFetchZyteOptions(currentFetchOptions); err != nil {
				return fmt.Errorf("update zyte fetch options: %w", err)
			}
			feed.FetchOptions = &newFetchOptions
		}
	}
	return nil
}

func updateFeed(ctx context.Context, edits FeedArgs, feedSvc *service.FeedService, feed *models.Feed) error {
	// Update the feed.
	if err := feedSvc.UpdateFeed(ctx, feed); err != nil {
		return fmt.Errorf("update feed: %w", err)
	}
	if edits.UpdateInterval != nil {
		// Delete scheduled job for feed.
		manager, err := scheduler.New()
		if err != nil {
			return fmt.Errorf("could not run scheduler: %w", err)
		}
		if err := manager.DeleteJob(
			quartz.NewJobKeyWithGroup(feed.GetID(), string(jobs.JobTypeUpdateFeed)),
		); err != nil {
			return fmt.Errorf("delete feed job: %w", err)
		}
		// Create a new job for the feed.
		newJob, err := jobs.NewUpdateFeedJob(ctx, feedSvc, feed.GetID())
		if err != nil {
			return fmt.Errorf("create new feed job: %w", err)
		}
		// Schedule the new job.
		if err = manager.ScheduleJob(newJob.JobDetail(), newJob.Trigger()); err != nil {
			return fmt.Errorf("schedule feed job: %w", err)
		}
		slogctx.Info(ctx, "Added new job for feed.",
			slog.String("job_id", newJob.JobDetail().JobKey().String()),
			slog.String("job_schedule", newJob.Trigger().Description()),
		)
	}

	if err := bulk.Flush(ctx); err != nil {
		slogctx.Warn(ctx, "Unable to flush updates.",
			slog.Any("error", err))
	}

	slogctx.FromCtx(ctx).Info("Feed updated.",
		slog.String("feed_id", feed.GetID()))

	return nil
}

// ClassifyFeedCmd is a command that will classify a feed.
type ClassifyFeedCmd struct {
	FeedID models.FeedID `help:"ID of feed" validate:"omitempty,required_without=FeedURL,startswith=feed_"`
}

func (c *ClassifyFeedCmd) Run() error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	if err := validation.Validate.Struct(c); err != nil {
		return fmt.Errorf("validate options: %w", err)
	}

	feedSvc, err := service.LoadFeedService()
	if err != nil {
		return fmt.Errorf("load feed service: %w", err)
	}

	details, err := feedSvc.GetFeed(ctx, c.FeedID)
	if err != nil {
		return fmt.Errorf("get feed: %w", err)
	}

	var feed *models.Feed
	switch details.FetchMethod {
	case models.FeedFetchMethodZyteArticles:
		feed, _, err = feedSvc.FetchFeedUpdatesAsArticles(ctx, details)
	case models.FeedFetchMethodDirect, models.FeedFetchMethodProxied:
		fallthrough
	default:
		feed, _, err = feedSvc.FetchFeedUpdates(ctx, details)
	}
	if err != nil {
		return fmt.Errorf("fetch feed updates: %w", err)
	}

	if err := service.ClassifyFeed(ctx, feed); err != nil {
		return fmt.Errorf("classify feed failed: %w", err)
	}

	return nil
}

// AddFeedCmd is a command that will add a custom feed with the given options.
type AddFeedCmd struct {
	FeedArgs `embed:""`

	URL         string                 `help:"URL of feed"             required:"" validate:"required,url"`
	FetchMethod models.FeedFetchMethod `help:"how the feed is fetched" required:"" validate:"required,oneof=direct proxied zyte-articles" enum:"direct,proxied,zyte-articles" default:"direct"`
}

func (c *AddFeedCmd) Run() error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	if err := validation.Validate.Struct(c); err != nil {
		return fmt.Errorf("validate options: %w", err)
	}

	feedSvc, err := service.LoadFeedService()
	if err != nil {
		return fmt.Errorf("load feed service: %w", err)
	}

	// Parse the given URL.
	feedURL, err := models.NormalizeFeedURL(c.URL)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}

	var (
		feed *models.Feed
	)

	switch c.FetchMethod {
	case models.FeedFetchMethodDirect:
		feed, err = feedSvc.FetchFeed(ctx, feedURL.String())
		if err != nil {
			return fmt.Errorf("fetch feed directly: %w", err)
		}
	case models.FeedFetchMethodProxied:
		feed, err = feedSvc.FetchFeed(ctx, feedURL.String(), service.FetchWithProxy(true))
		if err != nil {
			return fmt.Errorf("fetch feed with proxy: %w", err)
		}
	case models.FeedFetchMethodZyteArticles:
		// Parse the extraction options.
		extractFrom := zyte.ExtractFromHttpResponseBody
		if c.ZyteFetchArgs.ExtractFrom != nil {
			extractFrom = *c.ZyteFetchArgs.ExtractFrom
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
			return fmt.Errorf("fetch feed data with zyte: %w", err)
		}
		// Generate the feed from the Zyte response.
		feed, err = service.NewFeedFromZyteResponse(ctx, resp)
		if err != nil {
			return fmt.Errorf("generate feed from articles: %w", err)
		}
	}

	// Parse the given update interval and set it.
	if c.UpdateInterval != nil {
		updateInterval, err := time.ParseDuration(*c.UpdateInterval)
		if err != nil {
			return fmt.Errorf("parse update interval: %w", err)
		}
		feed.UpdateInterval = int64(updateInterval)
	}
	// Add any optionally specified values.
	if c.Name != nil {
		feed.Title = *c.Name
	}

	switch existing, err := feedSvc.GetFeed(ctx, feed.GetID()); {
	case err != nil && !errors.Is(err, models.ErrNotFound):
		return fmt.Errorf("check for existing feed: %w", err)
	case existing != nil:
		slogctx.Warn(ctx, "Feed already exists. Not adding.")
		return nil
	}

	// Add the new feed.
	if err := feedSvc.AddFeed(ctx, feed); err != nil {
		return fmt.Errorf("add feed: %w", err)
	}

	slogctx.Info(ctx, "Feed added.",
		slog.String("feed_id", feed.GetID()),
		slog.String("feed_url", feed.GetSourceURLs()[0]),
	)

	return nil
}

//nolint:funlen
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
