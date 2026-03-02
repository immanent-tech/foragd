// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package cli

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"

	"github.com/immanent-tech/foragd/models"
)

type FeedArgs struct {
	FeedID  models.UserID `arg:"" optional:"" help:"ID of feed"`
	FeedURL string        `arg:"" optional:"" help:"URL of feed"`
}

// FeedCmd contains sub commands for interacting with feeds.
type FeedCmd struct {
	Fetch FetchFeedCmd `cmd:"fetch" help:"fetch a feed (by either URL or ID)"`
}

// FetchFeedCmd is a command that will fetch a feed, by either URL or its Feed ID.
type FetchFeedCmd struct {
	FeedArgs

	Validate bool `default:"true" help:"validate the feed"`
}

func (c *FetchFeedCmd) Run() error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	var results *models.Feed

	switch {
	case c.FeedArgs.FeedID != "" && strings.HasPrefix(c.FeedArgs.FeedID, "feed_"):
		feed, err := models.GetFeedByID(ctx, c.FeedArgs.FeedID)
		if err != nil {
			return fmt.Errorf("get feed by id: %w", err)
		}
		results, err = models.NewFeedFromURL(ctx, feed.GetLink(), feed.GetID(), c.Validate)
		if err != nil {
			return fmt.Errorf("get feed by id: %w", err)
		}
	case c.FeedArgs.FeedURL != "":
		feedURL, err := url.Parse(c.FeedArgs.FeedURL)
		if err != nil {
			return fmt.Errorf("parse url: %w", err)
		}
		results, err = models.NewFeedFromURL(ctx, feedURL.String(), "", c.Validate)
		if err != nil {
			return fmt.Errorf("get feed by id: %w", err)
		}
	default:
		return errors.New("no ID or URL provided")
	}

	showFeedDetails(results)

	return nil
}

func showFeedDetails(feed *models.Feed) {
	fmt.Fprintf(os.Stdout, "Feed: %s\n", feed.GetTitle())
	fmt.Fprintf(os.Stdout, "Link: %s\n", feed.GetLink())
	fmt.Fprintf(os.Stdout, "Type: %s\n", feed.GetLink())
	fmt.Fprintf(os.Stdout, "Description: %s\n", feed.GetDescription())
	fmt.Fprintf(os.Stdout, "Updated: %s\n", feed.GetTimestamp())
	fmt.Fprintf(os.Stdout, "Categories: %v\n", feed.GetCategories())
	fmt.Fprintf(os.Stdout, "Image: %v\n", feed.GetImage())

	for article := range slices.Values(feed.GetItems()) {
		fmt.Fprintf(os.Stdout, "ID: %s\n", article.GetID())
		fmt.Fprintf(os.Stdout, "Title: %s\n", article.GetTitle())
		fmt.Fprintf(os.Stdout, "Link: %s\n", article.GetLink())
		fmt.Fprintf(os.Stdout, "Published: %s\n", article.GetTimestamp())
		fmt.Fprintf(os.Stdout, "Categories: %v\n", article.GetCategories())
		fmt.Fprintf(os.Stdout, "Description: %s\n", article.GetDescription())
		fmt.Fprintf(os.Stdout, "Content: %s\n", article.GetContent())
		fmt.Fprintf(os.Stdout, "\n")
	}
}
