// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/elastic/go-elasticsearch/v8/typedapi"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/config"
	"github.com/joshuar/go-feed-me/providers/elastic"
)

var validMigrations = []string{"feeds", "feeditems", "subscription", "users", "ingest", "scheduler", "session"}

var ErrMigrationFailed = errors.New("schema migration failed")

// Migration will create all necessary index templates settings and policies.
func Migration(ctx context.Context, api *typedapi.API, destructive bool, migrations ...string) error {
	// If no migrations are specified, perform migrations for all items.
	if slices.Contains(migrations, "all") {
		migrations = validMigrations
	}

	// Perform requested migrations.
	for migration := range slices.Values(migrations) {
		var err error

		switch migration {
		case "users":
			err = migrateUsers(ctx, api, destructive)
		case "feeds":
			err = migrateFeeds(ctx, api, destructive)
		case "feeditems":
			err = migrateFeedItems(ctx, api, destructive)
		case "subscriptions":
			err = migrateSubscriptions(ctx, api, destructive)
		case "scheduler":
			err = migrateScheduler(ctx, api, destructive)
		case "session":
			err = migrateSession(ctx, api, destructive)
		case "ingest":
			err = migrateIngest(ctx, api)
		}

		if err != nil {
			return errors.Join(ErrMigrationFailed, err)
		}
	}

	return nil
}

// migrateUsers contains migration actions for migrating users indices and
// settings.
func migrateUsers(ctx context.Context, api *typedapi.API, destructive bool) error {
	if err := elastic.PutComponentTemplate(ctx, api, UsersSchemaPrefix, NewComponentTemplateRequest(userComponentTemplate())); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}
	slogctx.FromCtx(ctx).Debug("Added users component template...")

	if err := elastic.PutIndexTemplate(ctx, api, UsersSchemaPrefix,
		NewIndexTemplateRequest(
			WithIndexPatterns(UsersSchemaPrefix+"_*"),
			WithComponentTemplates(UsersSchemaPrefix),
		),
	); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}
	slogctx.FromCtx(ctx).Debug("Added users index template...")

	userIndex := UsersSchemaPrefix + "_" + config.Environment()
	// Delete index if destructive set.
	if destructive {
		if _, err := api.Indices.Delete(userIndex).Do(ctx); err != nil && !ignoreErr(err) {
			return fmt.Errorf("%w: %w", ErrMigrationFailed, err)
		}
		slogctx.FromCtx(ctx).Debug("Deleted existing users index...")
	}
	// Make sure the index doesn't exist before continuing.
	found, err := api.Indices.Exists(userIndex).Do(ctx)
	if err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}
	// Create a job queue index if not found.
	if !found {
		_, err = elastic.NewIndexRequest(api, userIndex).Do(ctx)
		if err != nil {
			return errors.Join(ErrMigrationFailed, err)
		}
		slogctx.FromCtx(ctx).Debug("Created new users index...")
	}

	return nil
}

// migrateSubscriptions contains migration actions for migrating subscriptions indices and
// settings.
func migrateSubscriptions(ctx context.Context, api *typedapi.API, destructive bool) error {
	slogctx.FromCtx(ctx).Debug("Migrating subscriptions...")

	if err := elastic.PutComponentTemplate(ctx, api, SubscriptionsSchemaPrefix, NewComponentTemplateRequest(subscriptionsTemplate())); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := elastic.PutIndexTemplate(ctx, api, SubscriptionsSchemaPrefix,
		NewIndexTemplateRequest(
			WithIndexPatterns(SubscriptionsSchemaPrefix+"_*"),
			WithComponentTemplates(SubscriptionsSchemaPrefix),
		),
	); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	index := SubscriptionsSchemaPrefix + "_" + config.Environment()
	// Delete index if destructive set.
	if destructive {
		if _, err := api.Indices.Delete(index).Do(ctx); err != nil && !ignoreErr(err) {
			// return fmt.Errorf("%w: %w", ErrMigrationFailed, err)
		}
	}
	// Make sure the index doesn't exist before continuing.
	found, err := api.Indices.Exists(index).Do(ctx)
	if err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}
	// Create a job queue index if not found.
	if !found {
		_, err = elastic.NewIndexRequest(api, index).Do(ctx)
		if err != nil {
			return errors.Join(ErrMigrationFailed, err)
		}
	}

	return nil
}

// migrateFeeds contains migration actions for migrating feeds indices and
// settings.
func migrateFeeds(ctx context.Context, api *typedapi.API, destructive bool) error {
	slogctx.FromCtx(ctx).Debug("Migrating feeds...")

	if err := elastic.PutComponentTemplate(ctx, api, FeedsSchemaPrefix, NewComponentTemplateRequest(feedsComponentTemplate())); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := elastic.PutIndexTemplate(ctx, api, FeedsSchemaPrefix,
		NewIndexTemplateRequest(
			WithIndexPatterns(FeedsSchemaPrefix+"_*"),
			WithComponentTemplates(FeedsSchemaPrefix),
		),
	); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	feedsIndex := FeedsSchemaPrefix + "_" + config.Environment()
	// Delete index if destructive set.
	if destructive {
		if _, err := api.Indices.Delete(feedsIndex).Do(ctx); err != nil && !ignoreErr(err) {
			return fmt.Errorf("%w: %w", ErrMigrationFailed, err)
		}
	}
	// Make sure the index doesn't exist before continuing.
	found, err := api.Indices.Exists(feedsIndex).Do(ctx)
	if err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}
	// Create a job queue index if not found.
	if !found {
		_, err = elastic.NewIndexRequest(api, feedsIndex).Do(ctx)
		if err != nil {
			return errors.Join(ErrMigrationFailed, err)
		}
	}

	return nil
}

// migrateFeedItems contains migration actions for migrating feed items
// index mappings & settings and ILM policy.
func migrateFeedItems(ctx context.Context, api *typedapi.API, destructive bool) error {
	slogctx.FromCtx(ctx).Debug("Migrating feed items...")

	if destructive {
		if _, err := api.Indices.DeleteDataStream(ItemsSchemaPrefix + "_" + config.Environment()).Do(ctx); err != nil && !ignoreErr(err) {
			// return fmt.Errorf("%w: %w", ErrMigrationFailed, err)
		}
		if _, err := api.Ilm.DeleteLifecycle(ItemsSchemaPrefix).Do(ctx); err != nil && !ignoreErr(err) {
			return fmt.Errorf("%w: %w", ErrMigrationFailed, err)
		}
	}

	if _, err := api.Ilm.PutLifecycle(ItemsSchemaPrefix).Request(itemsILMPolicy()).Do(ctx); err != nil {
		return fmt.Errorf("%w: %w", ErrMigrationFailed, err)
	}

	if err := elastic.PutComponentTemplate(ctx, api, ItemsSchemaPrefix, NewComponentTemplateRequest(itemsComponentTemplate())); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := elastic.PutIndexTemplate(ctx, api, ItemsSchemaPrefix,
		NewIndexTemplateRequest(
			WithIndexPatterns(ItemsSchemaPrefix+"_*"),
			WithComponentTemplates(ItemsSchemaPrefix),
			WithPriority(500),
			AsDataStream(),
		),
	); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	return nil
}

// migrateScheduler contains migration actions for migrating scheduler
// index mappings & settings and ILM policy.
func migrateScheduler(ctx context.Context, api *typedapi.API, destructive bool) error {
	slogctx.FromCtx(ctx).Debug("Migrating feed items...")

	var (
		err   error
		found bool
	)

	// scheduler jobs indicies

	if err = elastic.PutComponentTemplate(ctx, api, SchedulerSchemaPrefix, NewComponentTemplateRequest(schedulerJobsComponentTemplate())); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err = elastic.PutIndexTemplate(ctx, api, SchedulerSchemaPrefix,
		NewIndexTemplateRequest(
			WithIndexPatterns(SchedulerSchemaPrefix+"_*"),
			WithComponentTemplates(SchedulerSchemaPrefix),
			WithPriority(500),
		),
	); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	jobsStateIndex := SchedulerSchemaPrefix + "_" + config.Environment()
	// Delete index if destructive set.
	if destructive {
		if _, err := api.Indices.Delete(jobsStateIndex).Do(ctx); err != nil && !ignoreErr(err) {
			return fmt.Errorf("%w: %w", ErrMigrationFailed, err)
		}
	}
	// Make sure the index doesn't exist before continuing.
	found, err = api.Indices.Exists(jobsStateIndex).Do(ctx)
	if err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}
	// Create a job queue index if not found.
	if !found {
		_, err = elastic.NewIndexRequest(api, jobsStateIndex).Do(ctx)
		if err != nil {
			return errors.Join(ErrMigrationFailed, err)
		}
	}

	return nil
}

// migrateSession contains migration actions for migrating session indices and
// settings.
func migrateSession(ctx context.Context, api *typedapi.API, destructive bool) error {
	slogctx.FromCtx(ctx).Debug("Migrating feeds...")

	if err := elastic.PutComponentTemplate(ctx, api, SessionsSchemaPrefix, NewComponentTemplateRequest(sessionsComponentTemplate())); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := elastic.PutIndexTemplate(ctx, api, SessionsSchemaPrefix,
		NewIndexTemplateRequest(
			WithIndexPatterns(SessionsSchemaPrefix+"_*"),
			WithComponentTemplates(SessionsSchemaPrefix),
			WithPriority(500),
		),
	); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	sessionIndex := SessionsSchemaPrefix + "_" + config.Environment()
	// Delete index if destructive set.
	if destructive {
		if _, err := api.Indices.Delete(sessionIndex).Do(ctx); err != nil && !ignoreErr(err) {
			return fmt.Errorf("%w: %w", ErrMigrationFailed, err)
		}
	}
	// Make sure the index doesn't exist before continuing.
	found, err := api.Indices.Exists(sessionIndex).Do(ctx)
	if err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}
	// Create a job queue index if not found.
	if !found {
		_, err = elastic.NewIndexRequest(api, sessionIndex).Do(ctx)
		if err != nil {
			return errors.Join(ErrMigrationFailed, err)
		}
	}

	return nil
}

// migrateIngest will migrate the ingest pipelines.
func migrateIngest(ctx context.Context, api *typedapi.API) error {
	slogctx.FromCtx(ctx).Debug("Migrating ingest pipeline...")

	if _, err := api.Ingest.DeletePipeline(ingestPipelineID).Do(ctx); err != nil && !ignoreErr(err) {
		return fmt.Errorf("%w: %w", ErrMigrationFailed, err)
	}
	slogctx.FromCtx(ctx).Debug("Deleted existing ingest pipeline.")

	if _, err := api.Ingest.PutPipeline(ingestPipelineID).Request(ingestPipelineFeeds()).Do(ctx); err != nil {
		return fmt.Errorf("%w: %w", ErrMigrationFailed, err)
	}
	slogctx.FromCtx(ctx).Debug("Added ingest pipeline.")

	return nil
}

func ignoreErr(err error) bool {
	var esErr types.ElasticsearchError
	if errors.Is(err, &esErr) {
		if esErr.Status != 404 {
			return true
		}
	}
	return false
}
