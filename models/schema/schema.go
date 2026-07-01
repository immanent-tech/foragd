// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/conflicts"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/ilm"
	"github.com/immanent-tech/foragd/providers/elastic/reindex"
	"github.com/immanent-tech/foragd/providers/elastic/templates"
)

// Option is a reusable generic function for applying options to a type.
type Option[T any] func(T)

var allILMPolicies = map[string]*ilm.Policy{
	"logs": logsILMPolicy,
}

// ILMOptions contains the options for performing ILM schema operations.
type ILMOptions struct {
	Policies []string `arg:"" default:"all" enum:"all,logs"`
}

// UpdateILMPolicies will update all the specified ILM policies.
func UpdateILMPolicies(ctx context.Context, api *elasticsearch.TypedClient, opts *ILMOptions) error {
	// If no migrations are specified, perform migrations for all items.
	if slices.Contains(opts.Policies, "all") {
		for name, policy := range allILMPolicies {
			if err := policy.Put(ctx, api); err != nil {
				return fmt.Errorf("update ILM policy %s: %w", name, err)
			}
			slogctx.FromCtx(ctx).Info("Updated ILM Policy.",
				slog.String("policy_name", name),
			)
		}
	} else {
		for name := range slices.Values(opts.Policies) {
			if policy, found := allILMPolicies[name]; !found {
				slogctx.FromCtx(ctx).Warn("No ILM policy with that name found.",
					slog.String("policy_name", name),
				)
			} else {
				if err := policy.Put(ctx, api); err != nil {
					return fmt.Errorf("update ILM policy %s: %w", name, err)
				}
				slogctx.FromCtx(ctx).Info("Updated ILM Policy.",
					slog.String("policy_name", name),
				)
			}
		}
	}

	return nil
}

var allIndices = []string{
	feedsIndexPrefix,
	itemsSchemaPrefix,
	favoritesSchemaPrefix,
	usersSchemaPrefix,
	subscriptionsSchemaPrefix,
	schedulerIndexPrefix,
	sessionsSchemaPrefix,
}

// IndicesOptions contains the options for performing index schema operations.
type IndicesOptions struct {
	Indices []string `arg:"" default:"all" enum:"all,feeds,items,favorites,users,subscriptions,scheduler,sessions" help:"List of indicies to perform command on."`
}

// CreateIndices creates indices and appropriate read/write aliases.
func CreateIndices(ctx context.Context, opts *IndicesOptions) error {
	// If no indices are specified, create indices for all items.
	if slices.Contains(opts.Indices, "all") {
		opts.Indices = allIndices
	}
	for prefix := range slices.Values(opts.Indices) {
		index := elastic.GenerateIndexName(prefix)
		writeAlias := prefix + "_" + config.GetEnvironment().String() + indexWriteSuffix
		readAlias := prefix + "_" + config.GetEnvironment().String() + indexReadSuffix
		// Create a scheduler index if one doesn't exist.
		if _, err := elastic.CreateIndexIfNotExists(ctx, prefix); err != nil {
			return fmt.Errorf("create index: %w", err)
		}
		// Add appropriate aliases.
		if err := elastic.UpdateIndexAlias(ctx, readAlias, index); err != nil {
			return fmt.Errorf("add read alias: %w", err)
		}
		if err := elastic.UpdateIndexAlias(ctx, writeAlias, index); err != nil {
			return fmt.Errorf("add write alias: %w", err)
		}
	}
	return nil
}

// UpdateIndicesSchema performs all requested schema migrations.
//
//nolint:maintidx // will not reduce size
func UpdateIndicesSchema(ctx context.Context, api *elasticsearch.TypedClient, opts *IndicesOptions) error {
	// If no migrations are specified, perform migrations for all items.
	if slices.Contains(opts.Indices, "all") {
		opts.Indices = allIndices
	}

	// Migrate Feed/Items common mappings component template.
	if err := migrateIndexTemplates(ctx, api, feedItemsCommonMappings, noDynamicMappingComponentTemplate); err != nil {
		return fmt.Errorf("could not migrate shared component templates: %w", err)
	}

	for index := range slices.Values(opts.Indices) {
		switch index {
		case itemsSchemaPrefix:
			// Create Items schemas.
			if err := migrateIndexTemplates(ctx, api,
				itemsComponentTemplate,
				itemsIndexTemplate,
				itemsILMPolicy,
			); err != nil {
				return fmt.Errorf("could not migrate items: %w", err)
			}
		case favoritesSchemaPrefix:
			// Create Favorites schemas.
			if err := migrateIndexTemplates(ctx, api,
				favoriteItemsComponentTemplate,
				favoriteItemsIndexTemplate,
			); err != nil {
				return fmt.Errorf("could not migrate favorite items: %w", err)
			}
		case feedsIndexPrefix:
			// Create Feeds schemas.
			if err := migrateIndexTemplates(ctx, api,
				feedsComponentTemplate,
				feedsIndexTemplate,
			); err != nil {
				return fmt.Errorf("could not migrate feeds: %w", err)
			}
		case usersSchemaPrefix:
			if err := migrateIndexTemplates(ctx, api,
				usersComponentTemplate,
				usersIndexTemplate,
			); err != nil {
				return fmt.Errorf("could not migrate users: %w", err)
			}
		case subscriptionsSchemaPrefix:
			if err := migrateIndexTemplates(ctx, api,
				subscriptionsComponentTemplate,
				subscriptionsIndexTemplate,
			); err != nil {
				return fmt.Errorf("could not migrate users: %w", err)
			}
		case schedulerIndexPrefix:
			if err := migrateIndexTemplates(ctx, api,
				schedulerComponentTemplate,
				schedulerIndexTemplate,
			); err != nil {
				return fmt.Errorf("could not migrate users: %w", err)
			}
		case sessionsSchemaPrefix:
			if err := migrateIndexTemplates(ctx, api,
				sessionsComponentTemplate,
				sessionsIndexTemplate,
			); err != nil {
				return fmt.Errorf("could not migrate sessions: %w", err)
			}
		}
	}

	return nil
}

type templatesMigration struct {
	componentTemplates []*templates.ComponentTemplate
	indexTemplate      *templates.IndexTemplate
	ilmPolicy          *ilm.Policy
}

type templateMigrationOption Option[*templatesMigration]

func withComponentTemplatesMigration(template *templates.ComponentTemplate) templateMigrationOption {
	return func(m *templatesMigration) {
		m.componentTemplates = append(m.componentTemplates, template)
	}
}

func withIndexTemplateMigration(template *templates.IndexTemplate) templateMigrationOption {
	return func(m *templatesMigration) {
		m.indexTemplate = template
	}
}

func withILMPolicyMigration(policy *ilm.Policy) templateMigrationOption {
	return func(m *templatesMigration) {
		m.ilmPolicy = policy
	}
}

// https://www.elastic.co/docs/manage-data/data-store/templates
func migrateIndexTemplates(
	ctx context.Context,
	api *elasticsearch.TypedClient,
	options ...templateMigrationOption,
) error {
	migration := &templatesMigration{}
	// Process migration options.
	for option := range slices.Values(options) {
		option(migration)
	}

	// Migrate component templates.
	if len(migration.componentTemplates) > 0 {
		for template := range slices.Values(migration.componentTemplates) {
			slogctx.FromCtx(ctx).Info("Migrating component template...",
				slog.String("name", template.Name))
			if err := template.Put(ctx, api); err != nil {
				return fmt.Errorf("could not migrate component template %s: %w", template.Name, err)
			}
		}
	}
	// Migrate index template.
	if migration.indexTemplate != nil {
		slogctx.FromCtx(ctx).Info("Migrating index template...",
			slog.String("name", migration.indexTemplate.Name))
		if err := migration.indexTemplate.Put(ctx, api); err != nil {
			return fmt.Errorf("could not migrate index template %s: %w", migration.indexTemplate.Name, err)
		}
	}
	// Migrate ILM policy.
	if migration.ilmPolicy != nil {
		slogctx.FromCtx(ctx).Info("Migrating ILM policy...",
			slog.String("name", migration.ilmPolicy.Name))
		if err := migration.ilmPolicy.Put(ctx, api); err != nil {
			return fmt.Errorf("could not migrate ilm policy %s: %w", migration.ilmPolicy.Name, err)
		}
	}

	return nil
}

// MigrateIndices performs all requested schema migrations.
func MigrateIndices(ctx context.Context, api *elasticsearch.TypedClient, opts *IndicesOptions) error {
	// If no migrations are specified, perform migrations for all items.
	if slices.Contains(opts.Indices, "all") {
		opts.Indices = allIndices
	}

	for index := range slices.Values(opts.Indices) {
		switch index {
		case itemsSchemaPrefix:
			if err := migrateIndexData(ctx, api, itemsSchemaPrefix); err != nil {
				return fmt.Errorf("migrate items: %w", err)
			}
		case usersSchemaPrefix:
			if err := migrateIndexData(ctx, api, usersSchemaPrefix); err != nil {
				return fmt.Errorf("migrate users: %w", err)
			}
			// ingest.NewIngestPipeline(
			// 	ingest.WithProcessor(types.ProcessorContainer{
			// 		Rename: &types.RenameProcessor{
			// 			Field:       "settings.max_history",
			// 			TargetField: "metadata.max_history",
			// 		},
			// 	}),
			// 	ingest.WithProcessor(types.ProcessorContainer{
			// 		Rename: &types.RenameProcessor{
			// 			Field:       "settings.updates_frequency",
			// 			TargetField: "metadata.updates_frequency",
			// 		},
			// 	}),
			// ),
		case favoritesSchemaPrefix:
			if err := migrateIndexData(ctx, api, favoritesSchemaPrefix); err != nil {
				return fmt.Errorf("migrate favorites: %w", err)
			}
		case schedulerIndexPrefix:
			if err := migrateIndexData(ctx, api, schedulerIndexPrefix); err != nil {
				return fmt.Errorf("migrate scheduler: %w", err)
			}
		// case feedStatusIndexPrefix:
		// 	err = migrateIndexData(ctx, api, feedStatusIndexPrefix, nil)
		case feedsIndexPrefix:
			if err := migrateIndexData(ctx, api, feedsIndexPrefix); err != nil {
				return fmt.Errorf("migrate feeds: %w", err)
			}
		case subscriptionsSchemaPrefix:
			if err := migrateIndexData(ctx, api, subscriptionsSchemaPrefix); err != nil {
				return fmt.Errorf("migrated subscriptions: %w", err)
			}
		case sessionsSchemaPrefix:
			if err := migrateIndexData(ctx, api, sessionsSchemaPrefix); err != nil {
				return fmt.Errorf("migrate sessions: %w", err)
			}
		}
	}
	return nil
}

func migrateIndexData(
	ctx context.Context,
	api *elasticsearch.TypedClient,
	prefix string,
) error {
	index := elastic.GenerateIndexName(prefix)
	writeAlias := prefix + "_" + config.GetEnvironment().String() + indexWriteSuffix
	readAlias := prefix + "_" + config.GetEnvironment().String() + indexReadSuffix

	// Create index.
	if _, err := elastic.CreateIndexIfNotExists(ctx, prefix); err != nil {
		return fmt.Errorf("could not create index %s: %w", index, err)
	}

	// Update the write alias.
	if err := elastic.UpdateIndexAlias(ctx, writeAlias, index); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	// Reindex if requested.
	if found, err := api.Indices.Exists(readAlias).Do(ctx); err != nil || !found {
		return fmt.Errorf("could not determine %s index state: %w", readAlias, err)
	}
	reindexResp, err := reindex.NewReindexOperation(api, reindex.NewSource(readAlias), reindex.NewDest(index, "")).
		WaitForCompletion(false).
		Conflicts(conflicts.Proceed).
		RequestsPerSecond("1000").
		Do(ctx)
	if err != nil {
		return fmt.Errorf("reindex: %w", err)
	}

	// Wait for the reindex to complete.
	if taskID := reindexResp.Task; taskID != nil {
		for {
			tasksResp, err := api.Tasks.Get(*taskID).Do(ctx)
			if err != nil {
				return fmt.Errorf("get tasks: %w", err)
			}

			if tasksResp.Completed {
				if tasksResp.Error != nil {
					return fmt.Errorf("reindex: %w", err)
				}
				slogctx.Info(ctx, "Reindex complete!")
				break
			}

			var status struct {
				Total            int `json:"total"`
				Created          int `json:"created"`
				Updated          int `json:"updated"`
				Deleted          int `json:"deleted"`
				Batches          int `json:"batches"`
				VersionConflicts int `json:"version_conflicts"`
			}

			if err := json.Unmarshal(tasksResp.Task.Status, &status); err != nil {
				slogctx.Warn(ctx, "Unable to parse task status.",
					slog.Any("error", err))
			} else {
				slogctx.Info(ctx, "Reindexing...",
					slog.String("task_id", *taskID),
					slog.Int("created", status.Created),
					slog.Int("updated", status.Updated),
					slog.Int("deleted", status.Deleted),
					slog.Int("version_conflicts", status.VersionConflicts),
					slog.Int("total", status.Total),
				)
			}
			time.Sleep(10 * time.Second)
		}
	} else {
		return errors.New("no reindex task")
	}

	// Update the read alias.
	if err = elastic.UpdateIndexAlias(ctx, readAlias, index); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	return nil
}
