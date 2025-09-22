// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/elastic/go-elasticsearch/v9"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/providers/elastic/reindex"
)

var validMigrations = []string{"users", "feeds", "items", "scheduler", "sessions", "logs", "ingest", "archive"}

// Migration will create all necessary index templates settings and policies.
func Migration(ctx context.Context, api *elasticsearch.TypedClient, destructive bool, migrations ...string) error {
	// If no migrations are specified, perform migrations for all items.
	if slices.Contains(migrations, "all") {
		migrations = validMigrations
	}

	migrationJobs, ctx := errgroup.WithContext(ctx)

	// Perform requested migrations.
	for migration := range slices.Values(migrations) {
		switch migration {
		case "users":
			migrationJobs.Go(func() error {
				return indexMigration(ctx, api, UsersSchemaPrefix, userComponentTemplate())
			})
		case "feeds":
			migrationJobs.Go(func() error {
				return indexMigration(ctx, api, FeedsSchemaPrefix, feedsComponentTemplate())
			})
		case "items":
			migrationJobs.Go(func() error {
				return migrateFeedItems(ctx, api)
			})
		case "archive":
			migrationJobs.Go(func() error {
				return indexMigration(ctx, api, ArticleArchiveSchemaPrefix, articleArchiveComponentTemplate())
			})
		case "scheduler":
			migrationJobs.Go(func() error {
				return indexMigration(ctx, api, SchedulerJobsPrefix, schedulerJobsComponentTemplate())
			})
			migrationJobs.Go(func() error {
				return indexMigration(ctx, api, SchedulerStatePrefix, schedulerStateComponentTemplate())
			})
		case "sessions":
			migrationJobs.Go(func() error {
				return indexMigration(ctx, api, SessionsSchemaPrefix, sessionsComponentTemplate())
			})
		case "logs":
			migrationJobs.Go(func() error {
				return migrateLogs(ctx, api, destructive)
			})
		case "ingest":
			migrationJobs.Go(func() error {
				return migrateIngest(ctx, api)
			})
		}
	}

	err := migrationJobs.Wait()
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	return nil
}

// indexMigration performs a migration of a standard index, including component & index templates as well as the index
// itself.
func indexMigration(ctx context.Context, api *elasticsearch.TypedClient, prefix string, schema *Template) error {
	schemaPrefix := prefix

	slogctx.FromCtx(ctx).Info("Performing appropriate migrations...",
		slog.String("schema", schemaPrefix))

	indexName := schemaPrefix + "_" + config.Version
	writeAlias := schemaPrefix + IndexWriteSuffix
	readAlias := schemaPrefix + IndexReadSuffix

	err := updateTemplates(ctx, api, prefix, schema, false)
	if err != nil {
		return fmt.Errorf("could not update %s templates: %w", prefix, err)
	}

	// Create index.
	found, err := api.Indices.Exists(indexName).Do(ctx)
	if err != nil {
		return fmt.Errorf("could not determine %s index state: %w", indexName, err)
	}
	if !found {
		slogctx.FromCtx(ctx).Info("Creating new index...",
			slog.String("name", indexName))
		_, err = api.Indices.Create(indexName).Do(ctx)
		if err != nil {
			return fmt.Errorf("could not create index %s: %w", indexName, err)
		}
	}

	// Update the write alias.
	err = updateAlias(ctx, api, writeAlias, indexName)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	slogctx.FromCtx(ctx).Info("Write alias updated.")

	// Reindex.
	reindexResp, err := reindex.NewReindexOperation(api, reindex.NewSource(readAlias), reindex.NewDest(indexName)).WaitForCompletion(true).Do(ctx)
	if err != nil {
		return fmt.Errorf("could not reindex: %w", err)
	}
	slogctx.FromCtx(ctx).Info("Reindex completed.",
		slog.String("src", prefix),
		slog.String("dest", indexName),
		slog.Int64("took", *reindexResp.Took),
	)

	// Update the read alias.
	err = updateAlias(ctx, api, readAlias, indexName)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	slogctx.FromCtx(ctx).Info("Read alias updated.")

	return nil
}

// migrateFeedItems contains migration actions for migrating items (datastream and archive).
func migrateFeedItems(ctx context.Context, api *elasticsearch.TypedClient) error {
	datastreamName := ItemsSchemaPrefix + "_" + config.Version
	writeAlias := ItemsSchemaPrefix + IndexWriteSuffix
	readAlias := ItemsSchemaPrefix + IndexReadSuffix

	slogctx.FromCtx(ctx).Debug("Migrating items datastream...")

	resp, err := api.Ilm.GetLifecycle().Do(ctx)
	if err != nil {
		return fmt.Errorf("unable to migrate items datastream: %w", err)
	}
	if _, found := resp[ItemsSchemaPrefix]; found {
		_, err := api.Ilm.DeleteLifecycle(ItemsSchemaPrefix).Do(ctx)
		if err != nil {
			return fmt.Errorf("unable to migrate items datastream: %w", err)
		}
	}
	_, err = api.Ilm.PutLifecycle(ItemsSchemaPrefix).Request(itemsILMPolicy()).Do(ctx)
	if err != nil {
		return fmt.Errorf("unable to migrate items datastream: %w", err)
	}
	slogctx.FromCtx(ctx).Info("Updated items datastream lifecycle policy.")

	err = updateTemplates(ctx, api, ItemsSchemaPrefix, itemsComponentTemplate(), true)
	if err != nil {
		return fmt.Errorf("unable to migrate items datastream: %w", err)
	}

	// Create datastream.
	_, err = api.Indices.CreateDataStream(datastreamName).Do(ctx)
	if err != nil {
		return fmt.Errorf("unable to create new datastream %s: %w", datastreamName, err)
	}

	// Update the write alias.
	err = updateAlias(ctx, api, writeAlias, datastreamName)
	if err != nil {
		return fmt.Errorf("unable to update items datastream write alias: %w", err)
	}
	slogctx.FromCtx(ctx).Info("Updated items datastream write alias.")

	// Reindex.
	reindexResp, err := reindex.NewReindexOperation(api, reindex.NewSource(readAlias), reindex.NewDest(datastreamName)).WaitForCompletion(true).Do(ctx)
	if err != nil {
		return fmt.Errorf("could not reindex: %w", err)
	}
	slogctx.FromCtx(ctx).Info("Completed items datastream reindex.",
		slog.Int64("took", *reindexResp.Took),
	)

	// Update the read alias.
	err = updateAlias(ctx, api, readAlias, datastreamName)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	slogctx.FromCtx(ctx).Info("Updated items datastream read alias.")

	return nil
}

// migrateLogs contains migration actions for migrating logs (datastream).
func migrateLogs(ctx context.Context, api *elasticsearch.TypedClient, destructive bool) error {
	componentTemplateName := LogsSchemaPrefix + "_component_template"
	indexTemplateName := LogsSchemaPrefix + "_index_template"

	slogctx.FromCtx(ctx).Debug("Migrating logs datastream...")
	// Create the logs datastream.
	if destructive {
		_, err := api.Indices.DeleteDataStream(LogsSchemaPrefix + "_" + config.Environment()).Do(ctx)
		if err != nil {
			return fmt.Errorf("unable to migrate items datastream: %w", err)
		}
		_, err = api.Ilm.DeleteLifecycle(LogsSchemaPrefix).Do(ctx)
		if err != nil {
			return fmt.Errorf("unable to migrate items datastream: %w", err)
		}
	}
	_, err := api.Ilm.PutLifecycle(LogsSchemaPrefix).Request(itemsILMPolicy()).Do(ctx)
	if err != nil {
		return fmt.Errorf("unable to migrate items datastream: %w", err)
	}
	componentTemplate := NewComponentTemplate(componentTemplateName, logsComponentTemplate(),
		WithComponentTemplateMetadata(defaultMetadata),
	)
	err = componentTemplate.Put(ctx, api)
	if err != nil {
		return fmt.Errorf("unable to migrate items datastream: %w", err)
	}
	indexTemplate := NewIndexTemplate(indexTemplateName,
		WithComponentTemplates(componentTemplateName),
		WithIndexPatterns(LogsSchemaPrefix+"_*"),
		AsDatastream(true),
		WithIndexTemplateMetadata(defaultMetadata),
	)
	err = indexTemplate.Put(ctx, api)
	if err != nil {
		return fmt.Errorf("unable to migrate items datastream: %w", err)
	}

	return nil
}

// migrateIngest will migrate the ingest pipelines.
func migrateIngest(ctx context.Context, api *elasticsearch.TypedClient) error {
	slogctx.FromCtx(ctx).Debug("Migrating ingest pipeline...")

	_, err := api.Ingest.DeletePipeline(ingestPipelineID).Do(ctx)
	if err != nil {
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

// updateAlias performs a swap of an alias to the given index. It adds the given index to the alias, sets it as the
// write destination, then removes any existing aliased indicies so the index remains as the only aliased one.
func updateAlias(ctx context.Context, api *elasticsearch.TypedClient, alias string, index string) error {
	_, err := api.Indices.PutAlias(index, alias).IsWriteIndex(true).Do(ctx)
	if err != nil {
		return fmt.Errorf("could not update alias %s to add index %s: %w", alias, index, err)
	}
	aliasesResp, err := api.Indices.GetAlias().Index(alias).Do(ctx)
	if err != nil {
		return fmt.Errorf("could not retrieve indices associated with alias %s: %w", alias, err)
	}
	for aliasedIndex := range aliasesResp {
		if aliasedIndex != index {
			// // Close the old index so writes don't happen.
			// _, err := api.Indices.Close(aliasedIndex).Do(ctx)
			// if err != nil {
			// 	slogctx.FromCtx(ctx).Warn("Could not close old index for alias removal.",
			// 		slog.String("index", aliasedIndex),
			// 		slog.Any("error", err))
			// 	continue
			// }
			// Remove the old index from the alias.
			_, err = api.Indices.DeleteAlias(aliasedIndex, alias).Do(ctx)
			if err != nil {
				return fmt.Errorf("unable to remove index %s from alias %s: %w", aliasedIndex, alias, err)
			}
		}
	}
	return nil
}

func updateTemplates(ctx context.Context, api *elasticsearch.TypedClient, prefix string, schema *Template, datastream bool) error {
	componentTemplateName := prefix + "_component_template"
	indexTemplateName := prefix + "_" + config.Version + "_index_template"

	slogctx.FromCtx(ctx).Info("Creating component template...",
		slog.String("name", componentTemplateName))
	componentTemplate := NewComponentTemplate(componentTemplateName, schema,
		WithComponentTemplateMetadata(defaultMetadata),
	)
	err := componentTemplate.Put(ctx, api)
	if err != nil {
		return fmt.Errorf("could not create component template %s: %w", componentTemplateName, err)
	}

	slogctx.FromCtx(ctx).Info("Creating index template...",
		slog.String("name", indexTemplateName))
	indexTemplate := NewIndexTemplate(indexTemplateName,
		WithComponentTemplates(componentTemplateName),
		WithIndexPatterns(prefix+"_"+config.Version),
		WithIndexTemplateMetadata(defaultMetadata),
		AsDatastream(datastream),
	)
	err = indexTemplate.Put(ctx, api)
	if err != nil {
		return fmt.Errorf("could not create index template %s: %w", indexTemplateName, err)
	}
	return nil
}
