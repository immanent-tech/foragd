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

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/components/config"
	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/providers/elastic/index"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/providers/elastic/schema"
)

// MigrateCmd defines the `migrate` command, which performs data-store migrations for schema changes.
type PruneCmd struct{}

// Run contains the logic for performing the migrate command.
func (r *PruneCmd) Run(_ *Arguments) error {
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	// Load the Elastic backend
	es, err := elastic.Connect(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to backend: %w", err)
	}

	users_index := schema.UsersSchemaPrefix
	ctx = elastic.UserIndexToCtx(ctx, users_index)
	feeds_index := schema.FeedsSchemaPrefix
	ctx = elastic.FeedsIndexToCtx(ctx, feeds_index)
	items_index := schema.FeedItemsSchemaPrefix + "_" + config.Environment()
	ctx = elastic.ItemsIndexToCtx(ctx, items_index)

	searchSize := 100
	pagination := make([]types.FieldValue, 0)

	// Get all users
	var users []*models.User
	for {
		var (
			data     []*models.User
			warnings error
		)

		resp, err := elastic.NewSearchRequest(es.GetAPI(),
			elastic.WithSearchIndex(users_index),
			elastic.WithSearchSize(searchSize),
			elastic.WithSearchAfter(pagination),
			elastic.WithSortOptions(elastic.SortByDocID("user_id")),
		).Do(ctx)
		if err != nil {
			return fmt.Errorf("prune failed: %w", err)
		}

		data, pagination, warnings = elastic.ExtractSourceFromHits[*models.User](resp.Hits.Hits)
		if warnings != nil {
			slogctx.FromCtx(ctx).Warn("Problems occurred while extracting source from docs.",
				slog.Any("warnings", err))
		}

		users = append(users, data...)

		// Stop if we are at the end of the results.
		if int(resp.Hits.Total.Value) < searchSize {
			break
		}
	}

	// Get all feeds from all users.
	var activeFeedIDs []models.FeedID
	for user := range slices.Values(users) {
		feedIDs := slices.Collect(maps.Keys(user.GetAllSubscriptionStatesByFeed()))
		activeFeedIDs = append(activeFeedIDs, feedIDs...)
	}

	// Prune any feeds that are not subscribed to by any user.
	resp, err := index.NewDeleteByQueryRequest(es.GetAPI(), feeds_index,
		index.WithDeleteQueryOptions(query.Bool(
			query.MustNot(query.FeedIDs(activeFeedIDs...)),
		)),
	).Do(ctx)
	if err != nil {
		return fmt.Errorf("prune failed: %w", err)
	}
	slogctx.FromCtx(ctx).Info("Feeds with no subscribers removed.",
		slog.Int64("count", *resp.Deleted),
	)

	// Prune items for feeds that are not subscribed to by any user.
	resp, err = index.NewDeleteByQueryRequest(es.GetAPI(), items_index,
		index.WithDeleteQueryOptions(query.Bool(
			query.MustNot(query.FeedIDs(activeFeedIDs...)),
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
