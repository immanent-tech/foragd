// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/providers/elastic/reindex"
)

// MigrationOptions contains the options for performing schema migrations.
type MigrationOptions struct {
	Indices   []string `arg:"" default:"all" enum:"all,feeds,items,favorites,users,scheduler,sessions" help:"List of indicies to perform command on."`
	NoReindex bool     `help:"Do not perform reindex from existing index."`
}

// PerformMigrations performs all requested schema migrations.
//
//nolint:gocognit,funlen,maintidx
func PerformMigrations(ctx context.Context, api *elasticsearch.TypedClient, opts *MigrationOptions) error {
	// If no migrations are specified, perform migrations for all items.
	if slices.Contains(opts.Indices, "all") {
		opts.Indices = []string{"users", "feeds", "items", "favorites", "scheduler", "sessions"}
	}

	// Migrate Feed/Items common mappings component template.
	err := migrateIndexTemplates(ctx, api,
		withComponentTemplatesMigration(
			NewComponentTemplate(
				"feed_items_common",
				NewTemplate(
					WithTemplateMapping(
						WithProperties(
							WithDatetimeMapping("published"),
							WithDatetimeMapping("updated"),
							WithTextMapping("title", &types.TextProperty{
								Type: "text",
								Fields: map[string]types.Property{
									"raw": types.NewKeywordProperty(),
									"exact": types.TextProperty{
										Analyzer: &EnglishExactAnalyzerName,
									},
									"search": types.NewSearchAsYouTypeProperty(),
								},
							}),
							WithTextMapping("description", nil),
							WithTextMapping("content", nil),
							WithTextMapping("authors", &types.TextProperty{
								Type: "text",
								Fields: map[string]types.Property{
									"raw": types.NewKeywordProperty(),
									"exact": types.TextProperty{
										Analyzer: &EnglishExactAnalyzerName,
									},
									"search": types.NewSearchAsYouTypeProperty(),
								},
							}),
							WithTextMapping("contributors", &types.TextProperty{
								Type: "text",
								Fields: map[string]types.Property{
									"raw": types.NewKeywordProperty(),
									"exact": types.TextProperty{
										Analyzer: &EnglishExactAnalyzerName,
									},
									"search": types.NewSearchAsYouTypeProperty(),
								},
							}),
							WithTextMapping("categories", &types.TextProperty{
								Type: "text",
								Fields: map[string]types.Property{
									"raw": types.NewKeywordProperty(),
									"exact": types.TextProperty{
										Analyzer: &EnglishExactAnalyzerName,
									},
									"search": types.NewSearchAsYouTypeProperty(),
								},
							}),
							WithKeywordMapping("language"),
							WithTextMapping("copyright", nil),
							WithKeywordMapping("source_type"),
							WithKeywordMapping("url"),
							WithObjectMapping("image",
								WithKeywordMapping("url"),
								WithTextMapping("title", nil),
							)),
						WithDynamicProperties(false),
					),
					WithTemplateSettings(
						WithAnalysis(types.IndexSettingsAnalysis{
							Analyzer: map[string]types.Analyzer{
								EnglishExactAnalyzerName: types.CustomAnalyzer{
									Tokenizer: "standard",
									Filter:    []string{"lowercase"},
								},
							},
						}),
					),
				),
				WithComponentTemplateMetadata(defaultMetadata),
			),
		),
	)
	if err != nil {
		return fmt.Errorf("could not migrate feed/items common mappings component template: %w", err)
	}

	for index := range slices.Values(opts.Indices) {
		switch index {
		case "items":
			componentTemplateName := "items_component_template"
			indexTemplateName := "items_index_template"
			indexPattern := "items-*"
			ilmPolicy := "items_ilm_policy"
			indexName := ItemsSchemaPrefix + "-" + config.Version + "-" + time.Now().Format("20060102")
			writeAlias := ItemsSchemaPrefix + IndexWriteSuffix
			readAlias := ItemsSchemaPrefix + IndexReadSuffix
			err := migrateIndexTemplates(ctx, api,
				// Items specific mappings component template.
				withComponentTemplatesMigration(
					NewComponentTemplate(
						componentTemplateName,
						NewTemplate(
							WithTemplateMapping(
								WithProperties(
									WithDatetimeMapping("@timestamp"),
									WithDatetimeMapping("created"),
									WithKeywordMapping("feed_id"),
									WithKeywordMapping("item_id"),
									WithTextMapping("feed_title", &types.TextProperty{
										Type: "text",
										Fields: map[string]types.Property{
											"raw": types.NewKeywordProperty(),
											"exact": types.TextProperty{
												Analyzer: &EnglishExactAnalyzerName,
											},
											"search": types.NewSearchAsYouTypeProperty(),
										},
									}),
									WithExistingMappings(FeedItemCommonMappings),
								),
								WithDynamicProperties(false),
							),
							WithTemplateSettings(
								WithAnalysis(types.IndexSettingsAnalysis{
									Analyzer: map[string]types.Analyzer{
										EnglishExactAnalyzerName: types.CustomAnalyzer{
											Tokenizer: "standard",
											Filter:    []string{"lowercase"},
										},
									},
								}),
								WithLifecycle(ilmPolicy, writeAlias),
							),
						),
					),
				),
				// Items index template.
				withIndexTemplateMigration(
					NewIndexTemplate(
						indexTemplateName,
						WithComponentTemplates("feed_items_common", componentTemplateName),
						WithIndexPatterns(indexPattern),
						WithIndexTemplateMetadata(defaultMetadata),
					),
				),
				// Items ILM Policy.
				withILMPolicyMigration(
					NewILMPolicy(
						ilmPolicy,
						WithPhase("hot",
							WithActions(WithRolloverMaxSize("50gb")),
						),
						WithPhase("warm",
							WithActions(
								WithShrinkToShards(1),
								WithForceMergeSegments(1),
							),
						),
						WithPhase("delete",
							WithMinAge("735d"),
							WithActions(WithDelete()),
						),
					),
				),
			)
			if err != nil {
				return fmt.Errorf("could not migrate items: %w", err)
			}
			err = migrateIndexData(ctx, api, indexName, writeAlias, readAlias, withNoReindex(opts.NoReindex))
			if err != nil {
				return fmt.Errorf("could not migrate items: %w", err)
			}
		case "favorites":
			componentTemplateName := "favorite_items_component_template"
			indexTemplateName := "favorite_items_index_template"
			indexPattern := "favorite-items-*"
			indexName := FavoriteItemsSchemaPrefix + "-" + config.Version
			writeAlias := FavoriteItemsSchemaPrefix + IndexWriteSuffix
			readAlias := FavoriteItemsSchemaPrefix + IndexReadSuffix
			err := migrateIndexTemplates(ctx, api,
				// Feeds specific mappings component template.
				withComponentTemplatesMigration(
					NewComponentTemplate(
						componentTemplateName,
						NewTemplate(
							WithTemplateMapping(
								WithProperties(
									WithDatetimeMapping("@timestamp"),
									WithDatetimeMapping("created"),
									WithKeywordMapping("feed_id"),
									WithKeywordMapping("item_id"),
									WithKeywordMapping("user_id"),
									WithKeywordMapping("subscription_id"),
								),
								WithDynamicProperties(false),
							),
							WithTemplateSettings(
								WithAnalysis(types.IndexSettingsAnalysis{
									Analyzer: map[string]types.Analyzer{
										EnglishExactAnalyzerName: types.CustomAnalyzer{
											Tokenizer: "standard",
											Filter:    []string{"lowercase"},
										},
									},
								}),
							),
						),
						WithComponentTemplateMetadata(defaultMetadata),
					),
				),
				// Feeds index template.
				withIndexTemplateMigration(
					NewIndexTemplate(
						indexTemplateName,
						WithComponentTemplates("feed_items_common", componentTemplateName),
						WithIndexPatterns(indexPattern),
						WithIndexTemplateMetadata(defaultMetadata),
					),
				),
			)
			if err != nil {
				return fmt.Errorf("could not migrate favorite items: %w", err)
			}
			err = migrateIndexData(ctx, api, indexName, writeAlias, readAlias, withNoReindex(opts.NoReindex))
			if err != nil {
				return fmt.Errorf("could not favorite items: %w", err)
			}
		case "feeds":
			componentTemplateName := "feeds_component_template"
			indexTemplateName := "feeds_index_template"
			indexPattern := "feeds-*"
			indexName := FeedsSchemaPrefix + "-" + config.Version
			writeAlias := FeedsSchemaPrefix + IndexWriteSuffix
			readAlias := FeedsSchemaPrefix + IndexReadSuffix
			err := migrateIndexTemplates(ctx, api,
				// Feeds specific mappings component template.
				withComponentTemplatesMigration(
					NewComponentTemplate(
						componentTemplateName,
						NewTemplate(
							WithTemplateMapping(
								WithProperties(
									WithKeywordMapping("feed_id"),
									WithDatetimeMapping("created_at"),
									WithDatetimeMapping("last_fetched"),
									WithKeywordMapping("source_urls"),
								),
								WithDynamicProperties(false),
							),
							WithTemplateSettings(
								WithAnalysis(types.IndexSettingsAnalysis{
									Analyzer: map[string]types.Analyzer{
										EnglishExactAnalyzerName: types.CustomAnalyzer{
											Tokenizer: "standard",
											Filter:    []string{"lowercase"},
										},
									},
								}),
							),
						),
						WithComponentTemplateMetadata(defaultMetadata),
					),
				),
				// Feeds index template.
				withIndexTemplateMigration(
					NewIndexTemplate(
						indexTemplateName,
						WithComponentTemplates("feed_items_common", componentTemplateName),
						WithIndexPatterns(indexPattern),
						WithIndexTemplateMetadata(defaultMetadata),
					),
				),
			)
			if err != nil {
				return fmt.Errorf("could not migrate feeds: %w", err)
			}
			err = migrateIndexData(ctx, api, indexName, writeAlias, readAlias, withNoReindex(opts.NoReindex))
			if err != nil {
				return fmt.Errorf("could not migrate feeds: %w", err)
			}
		case "users":
			componentTemplateName := "users_component_template"
			indexTemplateName := "users_index_template"
			indexPattern := "users-*"
			indexName := UsersSchemaPrefix + "-" + config.Version
			writeAlias := UsersSchemaPrefix + IndexWriteSuffix
			readAlias := UsersSchemaPrefix + IndexReadSuffix
			err := migrateIndexTemplates(ctx, api,
				// User specific mappings component template.
				withComponentTemplatesMigration(
					NewComponentTemplate(
						componentTemplateName,
						NewTemplate(
							WithTemplateMapping(
								WithProperties(
									WithKeywordMapping("user_id"),
									WithKeywordMapping("nickname"),
									WithKeywordMapping("avatar_url"),
									WithKeywordMapping("external_user_id"),
									WithKeywordMapping("email"),
									WithKeywordMapping("provider"),
									WithKeywordMapping("level"),
									WithDatetimeMapping("created_at"),
									WithDatetimeMapping("updated_at"),
									WithKeywordMapping("max_history"),
									WithFlattenedMapping("settings"),
									WithFlattenedMapping("subscriptions"),
									WithFlattenedMapping("favorites"),
								),
								WithDynamicProperties(false),
							),
						),
					),
				),
				// User index template.
				withIndexTemplateMigration(
					NewIndexTemplate(
						indexTemplateName,
						WithComponentTemplates(componentTemplateName),
						WithIndexPatterns(indexPattern),
						WithIndexTemplateMetadata(defaultMetadata),
					),
				),
			)
			if err != nil {
				return fmt.Errorf("could not migrate users: %w", err)
			}
			err = migrateIndexData(ctx, api, indexName, writeAlias, readAlias, withNoReindex(opts.NoReindex))
			if err != nil {
				return fmt.Errorf("could not migrate users: %w", err)
			}
		case "scheduler":
			componentTemplateName := "scheduler_component_template"
			indexTemplateName := "scheduler_index_template"
			indexPattern := "scheduler-*"
			indexName := SchedulerSchemaPrefix + "-" + config.Version
			writeAlias := SchedulerSchemaPrefix + IndexWriteSuffix
			readAlias := SchedulerSchemaPrefix + IndexReadSuffix
			err := migrateIndexTemplates(ctx, api,
				// User specific mappings component template.
				withComponentTemplatesMigration(
					NewComponentTemplate(
						componentTemplateName,
						NewTemplate(
							WithTemplateMapping(
								WithProperties(
									WithDatetimeMapping("updated_at"),
									WithFlattenedMapping("job_options"),
									WithFlattenedMapping("job_data"),
									WithKeywordMapping("job_type"),
									WithKeywordMapping("job_trigger_type"),
									WithFlattenedMapping("job_trigger"),
									WithDatetimeMapping("job_next_run"),
								),
								WithDynamicProperties(false),
							),
						),
					),
				),
				// User index template.
				withIndexTemplateMigration(
					NewIndexTemplate(
						indexTemplateName,
						WithComponentTemplates(componentTemplateName),
						WithIndexPatterns(indexPattern),
						WithIndexTemplateMetadata(defaultMetadata),
					),
				),
			)
			if err != nil {
				return fmt.Errorf("could not migrate users: %w", err)
			}
			err = migrateIndexData(ctx, api, indexName, writeAlias, readAlias, withNoReindex(opts.NoReindex))
			if err != nil {
				return fmt.Errorf("could not migrate users: %w", err)
			}
		case "sessions":
			componentTemplateName := "sessions_component_template"
			indexTemplateName := "sessions_index_template"
			indexPattern := "sessions-*"
			// ilmPolicy := "sessions_ilm_policy"
			// indexName := SessionsSchemaPrefix + "-" + time.Now().Format("20060102")
			indexName := SessionsSchemaPrefix + "-" + config.Version
			writeAlias := SessionsSchemaPrefix + IndexWriteSuffix
			readAlias := SessionsSchemaPrefix + IndexReadSuffix
			err := migrateIndexTemplates(ctx, api,
				// Sessions specific mappings component template.
				withComponentTemplatesMigration(
					NewComponentTemplate(
						componentTemplateName,
						NewTemplate(
							WithTemplateMapping(
								WithProperties(
									WithDatetimeMapping("expiry"),
									WithKeywordMapping("token"),
									WithBinaryMapping("data"),
								),
								WithDynamicProperties(false),
							),
							// WithTemplateSettings(
							// 	WithLifecycle(ilmPolicy, writeAlias),
							// ),
						),
					),
				),
				// Sessions index template.
				withIndexTemplateMigration(
					NewIndexTemplate(
						indexTemplateName,
						WithComponentTemplates(componentTemplateName),
						WithIndexPatterns(indexPattern),
						WithIndexTemplateMetadata(defaultMetadata),
					),
				),
				// // Sessions ILM Policy.
				// WithILMPolicyMigration(
				// 	NewILMPolicy(
				// 		ilmPolicy,
				// 		WithPhase("hot",
				// 			WithActions(WithRolloverMaxAge("30d")),
				// 		),
				// 		WithPhase("delete",
				// 			WithMinAge("1d"),
				// 			WithActions(WithDelete()),
				// 		),
				// 	),
				// ),
			)
			if err != nil {
				return fmt.Errorf("could not migrate sessions: %w", err)
			}
			err = migrateIndexData(ctx, api, indexName, writeAlias, readAlias, withNoReindex(opts.NoReindex))
			if err != nil {
				return fmt.Errorf("could not migrate sessions: %w", err)
			}
		}
	}

	return nil
}

type templatesMigration struct {
	componentTemplates []*ComponentTemplate
	indexTemplate      *IndexTemplate
	ilmPolicy          *ILMPolicy
}

type templateMigrationOption Option[*templatesMigration]

func withComponentTemplatesMigration(templates ...*ComponentTemplate) templateMigrationOption {
	return func(m *templatesMigration) {
		m.componentTemplates = templates
	}
}

func withIndexTemplateMigration(template *IndexTemplate) templateMigrationOption {
	return func(m *templatesMigration) {
		m.indexTemplate = template
	}
}

func withILMPolicyMigration(policy *ILMPolicy) templateMigrationOption {
	return func(m *templatesMigration) {
		m.ilmPolicy = policy
	}
}

// https://www.elastic.co/docs/manage-data/data-store/templates
func migrateIndexTemplates(ctx context.Context, api *elasticsearch.TypedClient, options ...templateMigrationOption) error {
	migration := &templatesMigration{}
	// Process migration options.
	for option := range slices.Values(options) {
		option(migration)
	}
	// Migrate component templates.
	if len(migration.componentTemplates) > 0 {
		for template := range slices.Values(migration.componentTemplates) {
			slogctx.FromCtx(ctx).Info("Migrating component template...",
				slog.String("name", template.name))
			err := template.Put(ctx, api)
			if err != nil {
				return fmt.Errorf("could not migrate component template %s: %w", template.name, err)
			}
		}
	}
	// Migrate index template.
	if migration.indexTemplate != nil {
		slogctx.FromCtx(ctx).Info("Migrating index template...",
			slog.String("name", migration.indexTemplate.name))
		err := migration.indexTemplate.Put(ctx, api)
		if err != nil {
			return fmt.Errorf("could not migrate index template %s: %w", migration.indexTemplate.name, err)
		}
	}
	// Migrate ILM policy.
	if migration.ilmPolicy != nil {
		slogctx.FromCtx(ctx).Info("Migrating ILM policy...",
			slog.String("name", migration.ilmPolicy.name))
		err := migration.ilmPolicy.Put(ctx, api)
		if err != nil {
			return fmt.Errorf("could not migrate ilm policy %s: %w", migration.ilmPolicy.name, err)
		}
	}
	return nil
}

type indexMigration struct {
	noReindex bool
}

type indexMigrationOption Option[*indexMigration]

func withNoReindex(noReindex bool) indexMigrationOption {
	return func(m *indexMigration) {
		m.noReindex = noReindex
	}
}

func migrateIndexData(ctx context.Context, api *elasticsearch.TypedClient, index, writeAlias, readAlias string, options ...indexMigrationOption) error {
	migration := &indexMigration{}
	// Process migration options.
	for option := range slices.Values(options) {
		option(migration)
	}

	// Create index.
	found, err := api.Indices.Exists(index).Do(ctx)
	if err != nil {
		return fmt.Errorf("could not determine %s index state: %w", index, err)
	}
	if !found {
		_, err = api.Indices.Create(index).Do(ctx)
		if err != nil {
			return fmt.Errorf("could not create index %s: %w", index, err)
		}
	}
	slogctx.FromCtx(ctx).Info("New index created",
		slog.String("name", index),
	)
	// Update the write alias.
	err = updateAlias(ctx, api, writeAlias, index)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	// Reindex if requested.
	found, err = api.Indices.Exists(readAlias).Do(ctx)
	if err != nil {
		return fmt.Errorf("could not determine %s index state: %w", readAlias, err)
	}
	if found && !migration.noReindex {
		reindexResp, err := reindex.NewReindexOperation(api, reindex.NewSource(readAlias), reindex.NewDest(index)).WaitForCompletion(true).Do(ctx)
		if err != nil {
			return fmt.Errorf("could not reindex: %w", err)
		}
		slogctx.FromCtx(ctx).Info("Reindex completed.",
			slog.String("src", readAlias),
			slog.String("dest", index),
			slog.Int64("took", *reindexResp.Took),
		)
	}
	// Update the read alias.
	err = updateAlias(ctx, api, readAlias, index)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	return nil
}

// updateAlias performs a swap of an alias to the given index. It adds the given index to the alias, sets it as the
// write destination, then removes any existing aliased indicies so the index remains as the only aliased one.
//
// https://www.elastic.co/docs/manage-data/data-store/aliases
func updateAlias(ctx context.Context, api *elasticsearch.TypedClient, alias string, index string) error {
	aliasesResp, err := api.Indices.GetAlias().Index(alias).Do(ctx)
	if err != nil {
		if getStatusCode(err) != http.StatusNotFound {
			return fmt.Errorf("could not retrieve indices associated with alias %s: %w", alias, err)
		}
	}
	// Remove existing index marked as write index from alias.
	for aliasedIndex, aliases := range aliasesResp {
		_, found := aliases.Aliases[alias]
		if found {
			_, err = api.Indices.DeleteAlias(aliasedIndex, alias).Do(ctx)
			if err != nil {
				return fmt.Errorf("unable to remove index %s from alias %s: %w", aliasedIndex, alias, err)
			}
			slogctx.FromCtx(ctx).Info("Removed index for alias.",
				slog.String("alias", alias),
				slog.String("old_index", aliasedIndex),
			)
		}
	}

	var writeIndex bool
	if strings.HasSuffix(alias, "rw") {
		// Set as write index if alias name ends in "rw".
		writeIndex = true
	}
	// Update the alias.
	_, err = api.Indices.PutAlias(index, alias).IsWriteIndex(writeIndex).Do(ctx)
	if err != nil {
		return fmt.Errorf("could not update alias %s to add index %s: %w", alias, index, err)
	}
	slogctx.FromCtx(ctx).Info("Index alias updated.",
		slog.String("alias", alias),
		slog.String("index", index),
		slog.Bool("is_write_index", writeIndex),
	)
	return nil
}

func getStatusCode(err error) int {
	var esErr *types.ElasticsearchError
	if errors.As(err, &esErr) {
		return esErr.Status
	}
	return http.StatusInternalServerError
}
