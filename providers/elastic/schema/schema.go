// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/config"
)

const (
	// FeedsIndexPrefix is a prefix used for feed related index/mapping/settings.
	FeedsIndexPrefix = "feeds"
	// ItemsSchemaPrefix is a prefix used for item related index/mapping/settings.
	ItemsSchemaPrefix = "items"
	// FavoriteItemsSchemaPrefix is a prefix used for item archive related index/mapping/settings.
	FavoriteItemsSchemaPrefix = "favorite-items"
	// UsersSchemaPrefix is a prefix used for user related index/mapping/settings.
	UsersSchemaPrefix = "users"
	// SubscriptionsSchemaPrefix is a prefix used for subscription related index/mapping/settings.
	SubscriptionsSchemaPrefix = "subscriptions"
	// SchedulerIndexPrefix is a prefix used for scheduler related index/mapping/settings.
	SchedulerIndexPrefix = "scheduler"
	// SessionsSchemaPrefix is a prefix used for sessions related index/mapping/settings.
	SessionsSchemaPrefix = "sessions"
	// LogsSchemaPrefix is a prefix used for application logs related index/mapping/settings.
	LogsSchemaPrefix = "application_logs"
	// IndexWriteSuffix is the suffix appended to indicies that are used for write (indexing) operations.
	IndexWriteSuffix = "_rw"
	// IndexReadSuffix is the suffix appended to indicies that are used for read (search, get) operations.
	IndexReadSuffix = "_ro"
)

var (
	EnglishExactAnalyzerName = "english_exact"
	// FeedItemCommonMappings are the mappings that are common across both feed and item objects.
	FeedItemCommonMappings = NewProperties()
	// defaultMetadata defines default metadata.
	defaultMetadata = types.Metadata{
		"version":    json.RawMessage(fmt.Sprintf("%q", config.Version)),
		"created_at": json.RawMessage(fmt.Sprintf("%q", time.Now().UTC().String())),
	}
)

// Option is a reusable generic function for applying options to a type.
type Option[T any] func(T)

// Options contains the options for performing schema migrations.
type Options struct {
	Indices []string `arg:"" default:"all" enum:"all,feeds,items,favorites,users,subscriptions,scheduler,sessions" help:"List of indicies to perform command on."`
}

// CreateSchemas performs all requested schema migrations.
//
//nolint:funlen,maintidx // will not reduce size
func CreateSchemas(ctx context.Context, api *elasticsearch.TypedClient, opts *Options) error {
	// If no migrations are specified, perform migrations for all items.
	if slices.Contains(opts.Indices, "all") {
		opts.Indices = []string{"users", "feeds", "items", "favorites", "scheduler", "sessions", "subscriptions"}
	}

	// Migrate Feed/Items common mappings component template.
	if err := migrateIndexTemplates(ctx, api,
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
	); err != nil {
		return fmt.Errorf("could not migrate feed/items common mappings component template: %w", err)
	}

	for index := range slices.Values(opts.Indices) {
		switch index {
		case "items":
			componentTemplateName := "items_component_template"
			indexTemplateName := "items_index_template"
			indexPattern := "items-*"
			ilmPolicy := "items_ilm_policy"
			// indexName := ItemsSchemaPrefix + "-" + config.Version + "-" + time.Now().Format("20060102")
			writeAlias := ItemsSchemaPrefix + IndexWriteSuffix
			// readAlias := ItemsSchemaPrefix + IndexReadSuffix
			if err := migrateIndexTemplates(ctx, api,
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
			); err != nil {
				return fmt.Errorf("could not migrate items: %w", err)
			}
			// err = migrateIndexData(ctx, api, indexName, writeAlias, readAlias, withNoReindex(opts.NoReindex))
			// if err != nil {
			// 	return fmt.Errorf("could not migrate items: %w", err)
			// }
		case "favorites":
			componentTemplateName := "favorite_items_component_template"
			indexTemplateName := "favorite_items_index_template"
			indexPattern := "favorite-items-*"
			// indexName := FavoriteItemsSchemaPrefix + "-" + config.Version
			// writeAlias := FavoriteItemsSchemaPrefix + IndexWriteSuffix
			// readAlias := FavoriteItemsSchemaPrefix + IndexReadSuffix
			if err := migrateIndexTemplates(ctx, api,
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
			); err != nil {
				return fmt.Errorf("could not migrate favorite items: %w", err)
			}
			// err = migrateIndexData(ctx, api, indexName, writeAlias, readAlias, withNoReindex(opts.NoReindex))
			// if err != nil {
			// 	return fmt.Errorf("could not favorite items: %w", err)
			// }
		case "feeds":
			componentTemplateName := "feeds_component_template"
			indexTemplateName := "feeds_index_template"
			indexPattern := "feeds-*"
			// indexName := FeedsSchemaPrefix + "-" + config.Version
			// writeAlias := FeedsSchemaPrefix + IndexWriteSuffix
			// readAlias := FeedsSchemaPrefix + IndexReadSuffix
			if err := migrateIndexTemplates(ctx, api,
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
			); err != nil {
				return fmt.Errorf("could not migrate feeds: %w", err)
			}
			// err = migrateIndexData(ctx, api, indexName, writeAlias, readAlias, withNoReindex(opts.NoReindex))
			// if err != nil {
			// 	return fmt.Errorf("could not migrate feeds: %w", err)
			// }
		case "users":
			componentTemplateName := "users_component_template"
			indexTemplateName := "users_index_template"
			indexPattern := "users-*"
			// indexName := UsersSchemaPrefix + "-" + config.Version
			// writeAlias := UsersSchemaPrefix + IndexWriteSuffix
			// readAlias := UsersSchemaPrefix + IndexReadSuffix
			if err := migrateIndexTemplates(ctx, api,
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
									WithFlattenedMapping("metadata"),
									WithKeywordMapping("item_favorites"),
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
			); err != nil {
				return fmt.Errorf("could not migrate users: %w", err)
			}
			// err = migrateIndexData(ctx, api, indexName, writeAlias, readAlias, withNoReindex(opts.NoReindex))
			// if err != nil {
			// 	return fmt.Errorf("could not migrate users: %w", err)
			// }
		case "subscriptions":
			componentTemplateName := "subscriptions_component_template"
			indexTemplateName := "subscriptions_index_template"
			indexPattern := "subscriptions-*"
			// indexName := UsersSchemaPrefix + "-" + config.Version
			// writeAlias := UsersSchemaPrefix + IndexWriteSuffix
			// readAlias := UsersSchemaPrefix + IndexReadSuffix
			if err := migrateIndexTemplates(ctx, api,
				// User specific mappings component template.
				withComponentTemplatesMigration(
					NewComponentTemplate(
						componentTemplateName,
						NewTemplate(
							WithTemplateMapping(
								WithProperties(
									WithKeywordMapping("subscription_id"),
									WithKeywordMapping("user_id"),
									WithKeywordMapping("type"),
									WithBooleanMapping("favorite"),
									WithDatetimeMapping("created_at"),
									WithDatetimeMapping("updated_at"),
									WithDatetimeMapping("marked_read_at"),
									WithObjectMapping("customisation",
										WithTextMapping("nickname", &types.TextProperty{
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
										WithKeywordMapping("image_url"),
									),
									WithFlattenedMapping("settings"),
									WithObjectMapping("feed_data",
										WithKeywordMapping("feed_id"),
										WithKeywordMapping("url"),
										WithFlattenedMapping("article_states"),
										WithFlattenedMapping("article_filters"),
									),
									WithObjectMapping("group_data",
										WithKeywordMapping("subscriptions"),
									),
									WithFlattenedMapping("search_data"),
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
			); err != nil {
				return fmt.Errorf("could not migrate users: %w", err)
			}
			// err = migrateIndexData(ctx, api, indexName, writeAlias, readAlias, withNoReindex(opts.NoReindex))
			// if err != nil {
			// 	return fmt.Errorf("could not migrate users: %w", err)
			// }
		case "scheduler":
			componentTemplateName := "scheduler_component_template"
			indexTemplateName := "scheduler_index_template"
			indexPattern := "scheduler-*"
			if err := migrateIndexTemplates(ctx, api,
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
									WithKeywordMapping("job_description"),
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
			); err != nil {
				return fmt.Errorf("could not migrate users: %w", err)
			}
			// err = migrateIndexData(ctx, api, indexName, writeAlias, readAlias, withNoReindex(opts.NoReindex))
			// if err != nil {
			// 	return fmt.Errorf("could not migrate users: %w", err)
			// }
		case "sessions":
			componentTemplateName := "sessions_component_template"
			indexTemplateName := "sessions_index_template"
			indexPattern := "sessions-*"
			// ilmPolicy := "sessions_ilm_policy"
			// indexName := SessionsSchemaPrefix + "-" + time.Now().Format("20060102")
			// indexName := SessionsSchemaPrefix + "-" + config.Version
			// writeAlias := SessionsSchemaPrefix + IndexWriteSuffix
			// readAlias := SessionsSchemaPrefix + IndexReadSuffix
			if err := migrateIndexTemplates(ctx, api,
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
			); err != nil {
				return fmt.Errorf("could not migrate sessions: %w", err)
			}
			// err = migrateIndexData(ctx, api, indexName, writeAlias, readAlias, withNoReindex(opts.NoReindex))
			// if err != nil {
			// 	return fmt.Errorf("could not migrate sessions: %w", err)
			// }
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
				slog.String("name", template.name))
			if err := template.Put(ctx, api); err != nil {
				return fmt.Errorf("could not migrate component template %s: %w", template.name, err)
			}
		}
	}
	// Migrate index template.
	if migration.indexTemplate != nil {
		slogctx.FromCtx(ctx).Info("Migrating index template...",
			slog.String("name", migration.indexTemplate.name))
		if err := migration.indexTemplate.Put(ctx, api); err != nil {
			return fmt.Errorf("could not migrate index template %s: %w", migration.indexTemplate.name, err)
		}
	}
	// Migrate ILM policy.
	if migration.ilmPolicy != nil {
		slogctx.FromCtx(ctx).Info("Migrating ILM policy...",
			slog.String("name", migration.ilmPolicy.name))
		if err := migration.ilmPolicy.Put(ctx, api); err != nil {
			return fmt.Errorf("could not migrate ilm policy %s: %w", migration.ilmPolicy.name, err)
		}
	}
	return nil
}

func getStatusCode(err error) int {
	var esErr *types.ElasticsearchError
	if errors.As(err, &esErr) {
		return esErr.Status
	}
	return http.StatusInternalServerError
}
