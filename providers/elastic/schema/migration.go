// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-feed-me/config"
	"github.com/immanent-tech/go-feed-me/providers/elastic"
)

var validMigrations = []string{"users", "feeds", "items", "scheduler", "sessions", "logs", "ingest"}

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
			err = indexMigration(ctx, api, UsersSchemaPrefix, userComponentTemplate(), destructive)
		case "feeds":
			err = indexMigration(ctx, api, FeedsSchemaPrefix, feedsComponentTemplate(), destructive)
		case "items":
			err = migrateFeedItems(ctx, api, destructive)
		case "scheduler":
			err = indexMigration(ctx, api, SchedulerJobsPrefix, schedulerJobsComponentTemplate(), destructive)
			if err == nil {
				err = indexMigration(ctx, api, SchedulerStatePrefix, schedulerStateComponentTemplate(), destructive)
			}
		case "sessions":
			err = indexMigration(ctx, api, SessionsSchemaPrefix, sessionsComponentTemplate(), destructive)
		case "logs":
			err = migrateLogs(ctx, api, destructive)
		case "ingest":
			err = migrateIngest(ctx, api)
		}

		if err != nil {
			return fmt.Errorf("failed running migration: %w", err)
		}
	}

	return nil
}

// indexMigration performs a migration of a standard index, including component & index templates as well as the index
// itself.
func indexMigration(ctx context.Context, api *elasticsearch.TypedClient, prefix string, schema types.IndexState, destructive bool) error {
	schemaPrefix := prefix

	slogctx.FromCtx(ctx).Debug("Performing appropriate migrations...",
		slog.String("schema", schemaPrefix))

	componentTemplate := schemaPrefix + "_component_template"
	indexTemplate := schemaPrefix + "_index_template"
	index := schemaPrefix + "_" + config.Environment()

	if destructive {
		slogctx.FromCtx(ctx).Debug("Deleting existing schemas...",
			slog.String("schema", schemaPrefix))
		// Check for and delete the index.
		found, err := api.Indices.Exists(index).Do(ctx)
		if err != nil {
			return fmt.Errorf("could not determine %s index state: %w", index, err)
		}
		if found {
			_, err := api.Indices.Delete(index).Do(ctx)
			if err != nil {
				return fmt.Errorf("could not delete index %s: %w", index, err)
			}
		}
		// Check for and delete the index template.
		found, err = api.Indices.ExistsIndexTemplate(indexTemplate).Do(ctx)
		if err != nil {
			return fmt.Errorf("could not determine %s index template state: %w", indexTemplate, err)
		}
		if found {
			_, err := api.Indices.DeleteIndexTemplate(indexTemplate).Do(ctx)
			if err != nil {
				return fmt.Errorf("could not delete component template %s: %w", indexTemplate, err)
			}
		}
		// Check for and delete the component template.
		found, err = api.Cluster.ExistsComponentTemplate(componentTemplate).Do(ctx)
		if err != nil {
			return fmt.Errorf("could not determine %s component template state: %w", index, err)
		}
		if found {
			_, err := api.Cluster.DeleteComponentTemplate(componentTemplate).Do(ctx)
			if err != nil {
				return fmt.Errorf("could not delete component template %s: %w", componentTemplate, err)
			}
		}
	}

	// Create component template.
	found, err := api.Cluster.ExistsComponentTemplate(componentTemplate).Do(ctx)
	if err != nil {
		return fmt.Errorf("could not determine %s component template state: %w", index, err)
	}
	if !found {
		slogctx.FromCtx(ctx).Debug("Creating component template...",
			slog.String("name", componentTemplate))
		err = elastic.PutComponentTemplate(ctx, api, componentTemplate, NewComponentTemplateRequest(schema))
		if err != nil {
			return fmt.Errorf("could not create component template %s: %w", componentTemplate, err)
		}
	}

	// Create index template.
	found, err = api.Indices.ExistsIndexTemplate(indexTemplate).Do(ctx)
	if err != nil {
		return fmt.Errorf("could not determine %s index template state: %w", indexTemplate, err)
	}
	if !found {
		slogctx.FromCtx(ctx).Debug("Creating index template...",
			slog.String("name", indexTemplate))
		err = elastic.PutIndexTemplate(ctx, api, indexTemplate,
			NewIndexTemplateRequest(
				WithIndexPatterns(prefix+"_*"),
				WithComponentTemplates(componentTemplate),
			),
		)
		if err != nil {
			return fmt.Errorf("could not create index template %s: %w", indexTemplate, err)
		}
	}

	// Create index.
	found, err = api.Indices.Exists(index).Do(ctx)
	if err != nil {
		return fmt.Errorf("could not determine %s index state: %w", index, err)
	}
	if !found {
		slogctx.FromCtx(ctx).Debug("Creating index...",
			slog.String("name", index))
		_, err = elastic.NewIndexRequest(api, index).Do(ctx)
		if err != nil {
			return fmt.Errorf("could not create index %s: %w", index, err)
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
	_, err := api.Ilm.PutLifecycle(ItemsSchemaPrefix).Request(itemsILMPolicy()).Do(ctx)
	if err != nil {
		return fmt.Errorf("unable to migrate items datastream: %w", err)
	}
	err = elastic.PutComponentTemplate(ctx, api, ItemsSchemaPrefix, NewComponentTemplateRequest(itemsComponentTemplate()))
	if err != nil {
		return fmt.Errorf("unable to migrate items datastream: %w", err)
	}
	err = elastic.PutIndexTemplate(ctx, api, ItemsSchemaPrefix,
		NewIndexTemplateRequest(
			WithIndexPatterns(ItemsSchemaPrefix+"_*"),
			WithComponentTemplates(ItemsSchemaPrefix),
			WithPriority(500),
			AsDataStream(),
		),
	)
	if err != nil {
		return fmt.Errorf("unable to migrate items datastream: %w", err)
	}

	slogctx.FromCtx(ctx).Debug("Migrating items archive...")
	// Create the items archive.
	err = elastic.PutComponentTemplate(ctx, api, ArticleArchiveSchemaPrefix, NewComponentTemplateRequest(articleArchiveComponentTemplate()))
	if err != nil {
		return fmt.Errorf("unable to migrate items archive: %w", err)
	}
	err = elastic.PutIndexTemplate(ctx, api, ArticleArchiveSchemaPrefix,
		NewIndexTemplateRequest(
			WithIndexPatterns(ArticleArchiveSchemaPrefix+"_*"),
			WithComponentTemplates(ArticleArchiveSchemaPrefix),
		),
	)
	if err != nil {
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
	_, err := api.Ilm.PutLifecycle(LogsSchemaPrefix).Request(itemsILMPolicy()).Do(ctx)
	if err != nil {
		return fmt.Errorf("unable to migrate items datastream: %w", err)
	}
	err = elastic.PutComponentTemplate(ctx, api, LogsSchemaPrefix, NewComponentTemplateRequest(logsComponentTemplate()))
	if err != nil {
		return fmt.Errorf("unable to migrate items datastream: %w", err)
	}
	err = elastic.PutIndexTemplate(ctx, api, LogsSchemaPrefix,
		NewIndexTemplateRequest(
			WithIndexPatterns(LogsSchemaPrefix+"_*"),
			WithComponentTemplates(LogsSchemaPrefix),
			WithPriority(500),
			AsDataStream(),
		),
	)
	if err != nil {
		return fmt.Errorf("unable to migrate logs datastream: %w", err)
	}

	return nil
}

// migrateIngest will migrate the ingest pipelines.
func migrateIngest(ctx context.Context, api *elasticsearch.TypedClient) error {
	slogctx.FromCtx(ctx).Debug("Migrating ingest pipeline...")

	_, err := api.Ingest.DeletePipeline(ingestPipelineID).Do(ctx)
	if err != nil && !elastic.ParseError(err).IsNotFound() {
		return fmt.Errorf("could not delete ingest pipeline: %w", err)
	}
	slogctx.FromCtx(ctx).Debug("Deleted existing ingest pipeline.")

	_, err = api.Ingest.PutPipeline(ingestPipelineID).Request(ingestPipelineFeeds()).Do(ctx)
	if err != nil {
		return fmt.Errorf("could not create ingest pipeline: %w", err)
	}
	slogctx.FromCtx(ctx).Debug("Added ingest pipeline.")

	return nil
}
