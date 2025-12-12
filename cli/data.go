// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/immanent-tech/foragd/providers/elastic"
)

// DataCmd defines the `data` command, which contains commands for manipulating data.
type DataCmd struct {
	Delete DeleteCmd `cmd:"delete" help:"Delete objects"`
}

type DeleteCmd struct {
	ObjectID string `arg:"" help:"ID of object to delete"`
}

func (c *DeleteCmd) Run(opts *DeleteCmd) error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()
	ctx = elastic.SetupIndexAliases(ctx)
	// Load the Elastic backend
	client, err := elastic.NewConnection()
	if err != nil {
		return fmt.Errorf("failed to connect to backend: %w", err)
	}
	switch {
	case strings.HasPrefix(opts.ObjectID, "feed_"):
		if err := client.DeleteFeed(ctx, opts.ObjectID); err != nil {
			return fmt.Errorf("unable to delete feed %s: %w", opts.ObjectID, err)
		}
	}

	return nil
}
