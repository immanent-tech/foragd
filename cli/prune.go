// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"os/signal"
	"slices"
	"syscall"

	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/config"
	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/providers/elastic/index"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/providers/elastic/schema"
)

var allOptions = []string{"feeds", "scheduler"}

// PruneCmd defines the command and options for the `prune` cli command.
type PruneCmd struct {
	Options []string `arg:"" default:"all" enum:"all,feeds,scheduler" help:"What things to prune."`
}

// Run contains the logic for performing the migrate command.
func (r *PruneCmd) Run(_ *Arguments) error {
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	// Load the Elastic backend
	api, err := elastic.Connect(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to backend: %w", err)
	}

	var options []string

	// If no migrations are specified, perform migrations for all items.
	if slices.Contains(r.Options, "all") {
		options = allOptions
	}

	// Perform requested migrations.
	for option := range slices.Values(options) {
		var err error
		switch option {
		case "feeds":
			err = pruneFeeds(ctx, api)
		case "scheduler":
			err = pruneScheduler(ctx, api)
		}
		if err != nil {
			return fmt.Errorf("prune failed: %w", err)
		}
	}

	return nil
}

func pruneFeeds(ctx context.Context, api *elastic.API) error {
	users_index := schema.UsersSchemaPrefix
	ctx = elastic.UserIndexToCtx(ctx, users_index)
	feeds_index := schema.FeedsSchemaPrefix
	ctx = elastic.FeedsIndexToCtx(ctx, feeds_index)
	items_index := schema.ItemsSchemaPrefix + "_" + config.Environment()
	ctx = elastic.ItemsIndexToCtx(ctx, items_index)

	searchSize := 100
	// searchPagination := make([]elastic.PaginationValue, 0)

	// Get all users
	var users []*models.User
	for {
		var (
			data []*models.User
			err  error
			// pagination  models.Pagination
			// searchAfter []types.FieldValue
		)

		data, _, err = elastic.Search[*models.User](ctx, api.GetAPI(), users_index, query.MatchAll(), searchSize)
		if err != nil {
			return fmt.Errorf("prune failed: %w", err)
		}

		// pagination, err = elastic.EncodePagination(searchAfter)
		// if err != nil {
		// 	return fmt.Errorf("prune failed: %w", err)
		// }
		// searchPagination, err = elastic.DecodePagination(pagination)
		// if err != nil {
		// 	return fmt.Errorf("prune failed: %w", err)
		// }

		users = append(users, data...)

		// Stop if we are at the end of the results.
		if len(data) < searchSize {
			break
		}
	}

	// Get all feeds from all users.
	var activeFeedIDs []models.FeedID
	for user := range slices.Values(users) {
		feedIDs := slices.Collect(maps.Keys(user.GetSubscriptionsByFeedID()))
		activeFeedIDs = append(activeFeedIDs, feedIDs...)
	}

	// Prune any feeds that are not subscribed to by any user.
	resp, err := index.NewDeleteByQueryRequest(api.GetAPI(), feeds_index,
		index.WithDeleteQueryOptions(query.Bool(
			query.MustNot(query.Terms("feed_id", activeFeedIDs...)),
		)),
	).Do(ctx)
	if err != nil {
		return fmt.Errorf("prune failed: %w", err)
	}
	slogctx.FromCtx(ctx).Info("Feeds with no subscribers removed.",
		slog.Int64("count", *resp.Deleted),
	)

	// Prune items for feeds that are not subscribed to by any user.
	resp, err = index.NewDeleteByQueryRequest(api.GetAPI(), items_index,
		index.WithDeleteQueryOptions(query.Bool(
			query.MustNot(query.Terms("feed_id", activeFeedIDs...)),
		)),
	).Do(ctx)
	if err != nil {
		return fmt.Errorf("prune failed: %w", err)
	}
	slogctx.FromCtx(ctx).Info("Items for feeds with no subscribers removed.",
		slog.Int64("count", *resp.Deleted),
	)

	return nil
}

func pruneScheduler(ctx context.Context, api *elastic.API) error {
	return nil
}
