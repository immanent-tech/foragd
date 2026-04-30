// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
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
	switch {
	case strings.HasPrefix(opts.ObjectID, "feed_"):
		if err := elastic.DeleteDoc(ctx, schema.FeedsIndexRW(), opts.ObjectID); err != nil {
			return fmt.Errorf("unable to delete feed %s: %w", opts.ObjectID, err)
		}
	case strings.HasPrefix(opts.ObjectID, "user_"):
		// Delete the user.
		if err := elastic.DeleteDoc(ctx, schema.UsersIndexRW(), opts.ObjectID); err != nil {
			return fmt.Errorf("unable to delete user %s: %w", opts.ObjectID, err)
		}
		// Delete the user's subscriptions.
		if err := elastic.DeleteDocs(
			ctx,
			schema.SubscriptionsIndexRW(),
			query.Term("user_id", opts.ObjectID),
		); err != nil {
			return fmt.Errorf("unable to delete user %s: %w", opts.ObjectID, err)
		}
		slogctx.FromCtx(ctx).Info("Deleted user.",
			slog.String("user_id", opts.ObjectID),
		)
	}

	return nil
}
