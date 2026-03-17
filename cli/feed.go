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

// FeedCmd contains sub commands for interacting with feeds.
type FeedCmd struct {
	Fetch FetchFeedCmd `cmd:"fetch" help:"fetch a feed (by either URL or ID)"`
}

// FetchFeedCmd is a command that will fetch a feed, by either URL or its Feed ID.
type FetchFeedCmd struct {
	FeedID   models.FeedID `help:"ID of feed"`
	FeedURL  string        `help:"URL of feed"`
	Validate bool          `help:"validate the feed" default:"false"`
}

func (c *FetchFeedCmd) Run() error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	var results *models.Feed

	switch {
	case c.FeedID != "" && strings.HasPrefix(c.FeedID, "feed_"):
		feed, err := models.GetFeedByID(ctx, c.FeedID)
		if err != nil {
			return fmt.Errorf("get feed by id: %w", err)
		}
		results, err = models.NewFeedFromURL(ctx, feed.GetLink(), feed.GetID(), c.Validate)
		if err != nil {
			return fmt.Errorf("get feed by id: %w", err)
		}
	case c.FeedURL != "":
		feedURL, err := url.Parse(c.FeedURL)
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
	var str strings.Builder

	str.WriteString("Feed: " + feed.GetTitle())
	str.WriteRune('\n')
	str.WriteString("Link: " + feed.GetLink())
	str.WriteRune('\n')
	str.WriteString("Type: " + string(feed.SourceType))
	str.WriteRune('\n')
	if feed.GetDescription() != "" {
		str.WriteString("Description:")
		str.WriteRune('\n')
		str.WriteString(feed.GetDescription())
		str.WriteRune('\n')
	}
	str.WriteString("Updated: " + feed.GetTimestamp().String())
	str.WriteRune('\n')
	if len(feed.GetCategories()) > 0 {
		str.WriteString("Categories: " + strings.Join(feed.GetCategories(), ","))
		str.WriteRune('\n')
	}
	if feed.GetImage() != nil {
		str.WriteString("Image: " + feed.GetImage().String())
	}
	str.WriteRune('\n')
	str.WriteRune('\n')

	for article := range slices.Values(feed.GetItems().SortByTimestamp()) {
		str.WriteString("---")
		str.WriteRune('\n')
		str.WriteString("Item ID: " + article.GetID())
		str.WriteRune('\n')
		str.WriteString("Title: " + article.GetTitle())
		str.WriteRune('\n')
		str.WriteString("Link: " + article.GetLink())
		str.WriteRune('\n')
		if article.GetDescription() != "" {
			str.WriteString("Description:")
			str.WriteRune('\n')
			str.WriteString(article.GetDescription())
			str.WriteRune('\n')
		}
		str.WriteString("Published: " + article.GetTimestamp().String())
		str.WriteRune('\n')
		if len(article.GetCategories()) > 0 {
			str.WriteString("Categories: " + strings.Join(article.GetCategories(), ","))
			str.WriteRune('\n')
		}
		if article.GetImage() != nil {
			str.WriteString("Image: " + article.GetImage().String())
		}
		if article.GetContent() != "" {
			str.WriteString("Content:")
			str.WriteRune('\n')
			str.WriteString(article.GetContent())
			str.WriteRune('\n')
		}
	}

	fmt.Fprintf(os.Stdout, "%s", str.String())
}
