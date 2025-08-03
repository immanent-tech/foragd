// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/elastic/go-elasticsearch/v9"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/config"
	"github.com/joshuar/go-feed-me/providers/elastic"
)

var validMigrations = []string{"users", "feeds", "items", "scheduler", "sessions", "logs", "ingest"}

var ErrMigrationFailed = errors.New("schema migration failed")

// Migration will create all necessary index templates settings and policies.
func Migration(ctx context.Context, api *elasticsearch.TypedClient, destructive bool, migrations ...string) error {
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
		case "items":
			err = migrateFeedItems(ctx, api, destructive)
		case "scheduler":
			err = migrateScheduler(ctx, api, destructive)
		case "sessions":
			err = migrateSession(ctx, api, destructive)
		case "logs":
			err = migrateLogs(ctx, api, destructive)
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
func migrateUsers(ctx context.Context, api *elasticsearch.TypedClient, destructive bool) error {
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
		_, err := api.Indices.Delete(userIndex).Do(ctx)
		if err != nil && !elastic.ParseError(err).IsNotFound() {
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

// migrateFeeds contains migration actions for migrating feeds indices and
// settings.
func migrateFeeds(ctx context.Context, api *elasticsearch.TypedClient, destructive bool) error {
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
		_, err := api.Indices.Delete(feedsIndex).Do(ctx)
		if err != nil && !elastic.ParseError(err).IsNotFound() {
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

// migrateFeedItems contains migration actions for migrating items (datastream and archive).
func migrateFeedItems(ctx context.Context, api *elasticsearch.TypedClient, destructive bool) error {
	slogctx.FromCtx(ctx).Debug("Migrating items datastream...")
	// Create the items datastream.
	if destructive {
		_, err := api.Indices.DeleteDataStream(ItemsSchemaPrefix + "_" + config.Environment()).Do(ctx)
		if err != nil && !elastic.ParseError(err).IsNotFound() {
			return fmt.Errorf("unable to migrate items datastream: %w", err)
		}
		_, err = api.Ilm.DeleteLifecycle(ItemsSchemaPrefix).Do(ctx)
		if err != nil && !elastic.ParseError(err).IsNotFound() {
			return fmt.Errorf("unable to migrate items datastream: %w", err)
		}
	}
	if _, err := api.Ilm.PutLifecycle(ItemsSchemaPrefix).Request(itemsILMPolicy()).Do(ctx); err != nil {
		return fmt.Errorf("unable to migrate items datastream: %w", err)
	}
	if err := elastic.PutComponentTemplate(ctx, api, ItemsSchemaPrefix, NewComponentTemplateRequest(itemsComponentTemplate())); err != nil {
		return fmt.Errorf("unable to migrate items datastream: %w", err)
	}
	if err := elastic.PutIndexTemplate(ctx, api, ItemsSchemaPrefix,
		NewIndexTemplateRequest(
			WithIndexPatterns(ItemsSchemaPrefix+"_*"),
			WithComponentTemplates(ItemsSchemaPrefix),
			WithPriority(500),
			AsDataStream(),
		),
	); err != nil {
		return fmt.Errorf("unable to migrate items datastream: %w", err)
	}

	slogctx.FromCtx(ctx).Debug("Migrating items archive...")
	// Create the items archive.
	if err := elastic.PutComponentTemplate(ctx, api, ArticleArchiveSchemaPrefix, NewComponentTemplateRequest(articleArchiveComponentTemplate())); err != nil {
		return fmt.Errorf("unable to migrate items archive: %w", err)
	}
	if err := elastic.PutIndexTemplate(ctx, api, ArticleArchiveSchemaPrefix,
		NewIndexTemplateRequest(
			WithIndexPatterns(ArticleArchiveSchemaPrefix+"_*"),
			WithComponentTemplates(ArticleArchiveSchemaPrefix),
		),
	); err != nil {
		return fmt.Errorf("unable to migrate items archive: %w", err)
	}
	archiveIndex := ArticleArchiveSchemaPrefix + "_" + config.Environment()
	// Delete index if destructive set.
	if destructive {
		_, err := api.Indices.Delete(archiveIndex).Do(ctx)
		if err != nil && !elastic.ParseError(err).IsNotFound() {
			return fmt.Errorf("unable to migrate items archive: %w", err)
		}
	}
	// Make sure the index doesn't exist before continuing.
	found, err := api.Indices.Exists(archiveIndex).Do(ctx)
	if err != nil {
		return fmt.Errorf("unable to migrate items archive: %w", err)
	}
	// Create a job queue index if not found.
	if !found {
		_, err = elastic.NewIndexRequest(api, archiveIndex).Do(ctx)
		if err != nil {
			return fmt.Errorf("unable to migrate items archive: %w", err)
		}
	}

	return nil
}

// migrateScheduler contains migration actions for migrating scheduler
// index mappings & settings and ILM policy.
func migrateScheduler(ctx context.Context, api *elasticsearch.TypedClient, destructive bool) error {
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
		_, err := api.Indices.Delete(jobsStateIndex).Do(ctx)
		if err != nil && !elastic.ParseError(err).IsNotFound() {
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
func migrateSession(ctx context.Context, api *elasticsearch.TypedClient, destructive bool) error {
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
		_, err := api.Indices.Delete(sessionIndex).Do(ctx)
		if err != nil && !elastic.ParseError(err).IsNotFound() {
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
func migrateIngest(ctx context.Context, api *elasticsearch.TypedClient) error {
	slogctx.FromCtx(ctx).Debug("Migrating ingest pipeline...")

	_, err := api.Ingest.DeletePipeline(ingestPipelineID).Do(ctx)
	if err != nil && !elastic.ParseError(err).IsNotFound() {
		return fmt.Errorf("%w: %w", ErrMigrationFailed, err)
	}
	slogctx.FromCtx(ctx).Debug("Deleted existing ingest pipeline.")

	if _, err := api.Ingest.PutPipeline(ingestPipelineID).Request(ingestPipelineFeeds()).Do(ctx); err != nil {
		return fmt.Errorf("%w: %w", ErrMigrationFailed, err)
	}
	slogctx.FromCtx(ctx).Debug("Added ingest pipeline.")

	return nil
}

// migrateLogs contains migration actions for migrating logs (datastream).
func migrateLogs(ctx context.Context, api *elasticsearch.TypedClient, destructive bool) error {
	slogctx.FromCtx(ctx).Debug("Migrating logs datastream...")
	// Create the logs datastream.
	if destructive {
		_, err := api.Indices.DeleteDataStream(LogsSchemaPrefix + "_" + config.Environment()).Do(ctx)
		if err != nil && !elastic.ParseError(err).IsNotFound() {
			return fmt.Errorf("unable to migrate items datastream: %w", err)
		}
		_, err = api.Ilm.DeleteLifecycle(LogsSchemaPrefix).Do(ctx)
		if err != nil && !elastic.ParseError(err).IsNotFound() {
			return fmt.Errorf("unable to migrate items datastream: %w", err)
		}
	}
	if _, err := api.Ilm.PutLifecycle(LogsSchemaPrefix).Request(itemsILMPolicy()).Do(ctx); err != nil {
		return fmt.Errorf("unable to migrate items datastream: %w", err)
	}
	if err := elastic.PutComponentTemplate(ctx, api, LogsSchemaPrefix, NewComponentTemplateRequest(logsComponentTemplate())); err != nil {
		return fmt.Errorf("unable to migrate items datastream: %w", err)
	}
	if err := elastic.PutIndexTemplate(ctx, api, LogsSchemaPrefix,
		NewIndexTemplateRequest(
			WithIndexPatterns(LogsSchemaPrefix+"_*"),
			WithComponentTemplates(LogsSchemaPrefix),
			WithPriority(500),
			AsDataStream(),
		),
	); err != nil {
		return fmt.Errorf("unable to migrate logs datastream: %w", err)
	}

	return nil
}
