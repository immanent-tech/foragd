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
	"github.com/immanent-tech/foragd/providers/elastic/ilm"
	"github.com/immanent-tech/foragd/providers/elastic/templates"
)

const (
	feedsIndexPrefix          = "feeds"
	itemsSchemaPrefix         = "items"
	favoriteItemsSchemaPrefix = "favorite-items"
	usersSchemaPrefix         = "users"
	subscriptionsSchemaPrefix = "subscriptions"
	schedulerIndexPrefix      = "scheduler"
	sessionsSchemaPrefix      = "sessions"

	// indexWriteSuffix is the suffix appended to indicies that are used for write (indexing) operations.
	indexWriteSuffix = "_rw"
	// indexReadSuffix is the suffix appended to indicies that are used for read (search, get) operations.
	indexReadSuffix = "_ro"
)

const (
	// FeedsIndexRO is the index alias for read-only access to feeds.
	FeedsIndexRO = feedsIndexPrefix + indexReadSuffix
	// FeedsIndexRW is the index alias for read-write access to feeds.
	FeedsIndexRW = feedsIndexPrefix + indexWriteSuffix
	// ItemsIndexRO is the index alias for read-only access to items.
	ItemsIndexRO = itemsSchemaPrefix + indexReadSuffix
	// ItemsIndexRW is the index alias for read-write access to items.
	ItemsIndexRW = itemsSchemaPrefix + indexWriteSuffix
	// SubscriptionsIndexRO is the index alias for read-only access to subscriptions.
	SubscriptionsIndexRO = subscriptionsSchemaPrefix + indexReadSuffix
	// SubscriptionsIndexRW is the index alias for read-write access to subscriptions.
	SubscriptionsIndexRW = subscriptionsSchemaPrefix + indexWriteSuffix
	// FavoriteArticlesIndexRO is the index alias for read-only access to subscriptions.
	FavoriteArticlesIndexRO = favoriteItemsSchemaPrefix + indexReadSuffix
	// FavoriteArticlesIndexRW is the index alias for read-only access to subscriptions.
	FavoriteArticlesIndexRW = favoriteItemsSchemaPrefix + indexWriteSuffix
	// UsersIndexRO is the index alias for read-only access to users.
	UsersIndexRO = usersSchemaPrefix + indexReadSuffix
	// UsersIndexRW is the index alias for read-write access to users.
	UsersIndexRW = usersSchemaPrefix + indexWriteSuffix
	// SessionsIndexRO is the index alias for read-only access to session data.
	SessionsIndexRO = sessionsSchemaPrefix + indexReadSuffix
	// SessionsIndexRW is the index alias for read-write access to session data.
	SessionsIndexRW = sessionsSchemaPrefix + indexWriteSuffix
	// SchedulerIndexRO is the index alias for read-only access to scheduler data.
	SchedulerIndexRO = schedulerIndexPrefix + indexReadSuffix
	// SchedulerIndexRW is the index alias for read-write access to scheduler data.
	SchedulerIndexRW = schedulerIndexPrefix + indexWriteSuffix
)

var (
	EnglishExactAnalyzerName = "english_exact"
	// FeedItemCommonMappings are the mappings that are common across both feed and item objects.
	FeedItemCommonMappings = templates.NewProperties()
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
			templates.NewComponentTemplate(
				"feed_items_common",
				templates.NewTemplate(
					templates.WithTemplateMapping(
						templates.WithProperties(
							templates.WithDatetimeMapping("published"),
							templates.WithDatetimeMapping("updated"),
							templates.WithTextMapping("title", &types.TextProperty{
								Type: "text",
								Fields: map[string]types.Property{
									"raw": types.NewKeywordProperty(),
									"exact": types.TextProperty{
										Analyzer: &EnglishExactAnalyzerName,
									},
									"search": types.NewSearchAsYouTypeProperty(),
								},
							}),
							templates.WithTextMapping("description", nil),
							templates.WithTextMapping("content", nil),
							templates.WithTextMapping("authors", &types.TextProperty{
								Type: "text",
								Fields: map[string]types.Property{
									"raw": types.NewKeywordProperty(),
									"exact": types.TextProperty{
										Analyzer: &EnglishExactAnalyzerName,
									},
									"search": types.NewSearchAsYouTypeProperty(),
								},
							}),
							templates.WithTextMapping("contributors", &types.TextProperty{
								Type: "text",
								Fields: map[string]types.Property{
									"raw": types.NewKeywordProperty(),
									"exact": types.TextProperty{
										Analyzer: &EnglishExactAnalyzerName,
									},
									"search": types.NewSearchAsYouTypeProperty(),
								},
							}),
							templates.WithTextMapping("categories", &types.TextProperty{
								Type: "text",
								Fields: map[string]types.Property{
									"raw": types.NewKeywordProperty(),
									"exact": types.TextProperty{
										Analyzer: &EnglishExactAnalyzerName,
									},
									"search": types.NewSearchAsYouTypeProperty(),
								},
							}),
							templates.WithKeywordMapping("language"),
							templates.WithTextMapping("copyright", nil),
							templates.WithKeywordMapping("source_type"),
							templates.WithKeywordMapping("url"),
							templates.WithObjectMapping("image",
								templates.WithKeywordMapping("url"),
								templates.WithTextMapping("title", nil),
							)),
						templates.WithDynamicProperties(false),
					),
					templates.WithTemplateSettings(
						templates.WithAnalysis(types.IndexSettingsAnalysis{
							Analyzer: map[string]types.Analyzer{
								EnglishExactAnalyzerName: types.CustomAnalyzer{
									Tokenizer: "standard",
									Filter:    []string{"lowercase"},
								},
							},
						}),
					),
				),
				templates.WithComponentTemplateMetadata(defaultMetadata),
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
			writeAlias := itemsSchemaPrefix + indexWriteSuffix
			// readAlias := ItemsSchemaPrefix + IndexReadSuffix
			if err := migrateIndexTemplates(ctx, api,
				// Items specific mappings component template.
				withComponentTemplatesMigration(
					templates.NewComponentTemplate(
						componentTemplateName,
						templates.NewTemplate(
							templates.WithTemplateMapping(
								templates.WithProperties(
									templates.WithDatetimeMapping("@timestamp"),
									templates.WithDatetimeMapping("created"),
									templates.WithKeywordMapping("feed_id"),
									templates.WithKeywordMapping("item_id"),
									templates.WithTextMapping("feed_title", &types.TextProperty{
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
								templates.WithDynamicProperties(false),
							),
							templates.WithTemplateSettings(
								templates.WithAnalysis(types.IndexSettingsAnalysis{
									Analyzer: map[string]types.Analyzer{
										EnglishExactAnalyzerName: types.CustomAnalyzer{
											Tokenizer: "standard",
											Filter:    []string{"lowercase"},
										},
									},
								}),
								templates.WithLifecycle(ilmPolicy, writeAlias),
							),
						),
					),
				),
				// Items index template.
				withIndexTemplateMigration(
					templates.NewIndexTemplate(
						indexTemplateName,
						templates.WithComponentTemplates("feed_items_common", componentTemplateName),
						templates.WithIndexPatterns(indexPattern),
						templates.WithIndexTemplateMetadata(defaultMetadata),
					),
				),
				// Items ILM Policy.
				withILMPolicyMigration(
					ilm.NewILMPolicy(
						ilmPolicy,
						ilm.WithPhase("hot",
							ilm.WithActions(ilm.WithRolloverMaxSize("50gb")),
						),
						ilm.WithPhase("warm",
							ilm.WithActions(
								ilm.WithShrinkToShards(1),
								ilm.WithForceMergeSegments(1),
							),
						),
						ilm.WithPhase("delete",
							ilm.WithMinAge("735d"),
							ilm.WithActions(ilm.WithDelete()),
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
					templates.NewComponentTemplate(
						componentTemplateName,
						templates.NewTemplate(
							templates.WithTemplateMapping(
								templates.WithProperties(
									templates.WithDatetimeMapping("@timestamp"),
									templates.WithDatetimeMapping("created"),
									templates.WithKeywordMapping("feed_id"),
									templates.WithKeywordMapping("item_id"),
									templates.WithKeywordMapping("user_id"),
									templates.WithKeywordMapping("subscription_id"),
								),
								templates.WithDynamicProperties(false),
							),
							templates.WithTemplateSettings(
								templates.WithAnalysis(types.IndexSettingsAnalysis{
									Analyzer: map[string]types.Analyzer{
										EnglishExactAnalyzerName: types.CustomAnalyzer{
											Tokenizer: "standard",
											Filter:    []string{"lowercase"},
										},
									},
								}),
							),
						),
						templates.WithComponentTemplateMetadata(defaultMetadata),
					),
				),
				// Feeds index template.
				withIndexTemplateMigration(
					templates.NewIndexTemplate(
						indexTemplateName,
						templates.WithComponentTemplates("feed_items_common", componentTemplateName),
						templates.WithIndexPatterns(indexPattern),
						templates.WithIndexTemplateMetadata(defaultMetadata),
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
					templates.NewComponentTemplate(
						componentTemplateName,
						templates.NewTemplate(
							templates.WithTemplateMapping(
								templates.WithProperties(
									templates.WithKeywordMapping("feed_id"),
									templates.WithDatetimeMapping("created_at"),
									templates.WithDatetimeMapping("last_fetched"),
									templates.WithKeywordMapping("source_urls"),
								),
								templates.WithDynamicProperties(false),
							),
							templates.WithTemplateSettings(
								templates.WithAnalysis(types.IndexSettingsAnalysis{
									Analyzer: map[string]types.Analyzer{
										EnglishExactAnalyzerName: types.CustomAnalyzer{
											Tokenizer: "standard",
											Filter:    []string{"lowercase"},
										},
									},
								}),
							),
						),
						templates.WithComponentTemplateMetadata(defaultMetadata),
					),
				),
				// Feeds index template.
				withIndexTemplateMigration(
					templates.NewIndexTemplate(
						indexTemplateName,
						templates.WithComponentTemplates("feed_items_common", componentTemplateName),
						templates.WithIndexPatterns(indexPattern),
						templates.WithIndexTemplateMetadata(defaultMetadata),
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
					templates.NewComponentTemplate(
						componentTemplateName,
						templates.NewTemplate(
							templates.WithTemplateMapping(
								templates.WithProperties(
									templates.WithKeywordMapping("user_id"),
									templates.WithKeywordMapping("nickname"),
									templates.WithKeywordMapping("avatar_url"),
									templates.WithKeywordMapping("external_user_id"),
									templates.WithKeywordMapping("email"),
									templates.WithKeywordMapping("provider"),
									templates.WithKeywordMapping("level"),
									templates.WithDatetimeMapping("created_at"),
									templates.WithDatetimeMapping("updated_at"),
									templates.WithKeywordMapping("max_history"),
									templates.WithFlattenedMapping("settings"),
									templates.WithFlattenedMapping("metadata"),
									templates.WithKeywordMapping("item_favorites"),
								),
								templates.WithDynamicProperties(false),
							),
						),
					),
				),
				// User index template.
				withIndexTemplateMigration(
					templates.NewIndexTemplate(
						indexTemplateName,
						templates.WithComponentTemplates(componentTemplateName),
						templates.WithIndexPatterns(indexPattern),
						templates.WithIndexTemplateMetadata(defaultMetadata),
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
					templates.NewComponentTemplate(
						componentTemplateName,
						templates.NewTemplate(
							templates.WithTemplateMapping(
								templates.WithProperties(
									templates.WithKeywordMapping("subscription_id"),
									templates.WithKeywordMapping("user_id"),
									templates.WithKeywordMapping("type"),
									templates.WithBooleanMapping("favorite"),
									templates.WithDatetimeMapping("created_at"),
									templates.WithDatetimeMapping("updated_at"),
									templates.WithDatetimeMapping("marked_read_at"),
									templates.WithObjectMapping("customisation",
										templates.WithTextMapping("nickname", &types.TextProperty{
											Type: "text",
											Fields: map[string]types.Property{
												"raw": types.NewKeywordProperty(),
												"exact": types.TextProperty{
													Analyzer: &EnglishExactAnalyzerName,
												},
												"search": types.NewSearchAsYouTypeProperty(),
											},
										}),
										templates.WithTextMapping("categories", &types.TextProperty{
											Type: "text",
											Fields: map[string]types.Property{
												"raw": types.NewKeywordProperty(),
												"exact": types.TextProperty{
													Analyzer: &EnglishExactAnalyzerName,
												},
												"search": types.NewSearchAsYouTypeProperty(),
											},
										}),
										templates.WithKeywordMapping("image_url"),
									),
									templates.WithFlattenedMapping("settings"),
									templates.WithObjectMapping("feed_data",
										templates.WithKeywordMapping("feed_id"),
										templates.WithKeywordMapping("url"),
										templates.WithFlattenedMapping("article_states"),
										templates.WithFlattenedMapping("article_filters"),
									),
									templates.WithObjectMapping("group_data",
										templates.WithKeywordMapping("subscriptions"),
									),
									templates.WithFlattenedMapping("search_data"),
								),
								templates.WithDynamicProperties(false),
							),
							templates.WithTemplateSettings(
								templates.WithAnalysis(types.IndexSettingsAnalysis{
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
					templates.NewIndexTemplate(
						indexTemplateName,
						templates.WithComponentTemplates(componentTemplateName),
						templates.WithIndexPatterns(indexPattern),
						templates.WithIndexTemplateMetadata(defaultMetadata),
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
					templates.NewComponentTemplate(
						componentTemplateName,
						templates.NewTemplate(
							templates.WithTemplateMapping(
								templates.WithProperties(
									templates.WithDatetimeMapping("updated_at"),
									templates.WithFlattenedMapping("job_options"),
									templates.WithFlattenedMapping("job_data"),
									templates.WithKeywordMapping("job_type"),
									templates.WithKeywordMapping("job_description"),
									templates.WithKeywordMapping("job_trigger_type"),
									templates.WithFlattenedMapping("job_trigger"),
									templates.WithDatetimeMapping("job_next_run"),
								),
								templates.WithDynamicProperties(false),
							),
						),
					),
				),
				// User index template.
				withIndexTemplateMigration(
					templates.NewIndexTemplate(
						indexTemplateName,
						templates.WithComponentTemplates(componentTemplateName),
						templates.WithIndexPatterns(indexPattern),
						templates.WithIndexTemplateMetadata(defaultMetadata),
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
					templates.NewComponentTemplate(
						componentTemplateName,
						templates.NewTemplate(
							templates.WithTemplateMapping(
								templates.WithProperties(
									templates.WithDatetimeMapping("expiry"),
									templates.WithKeywordMapping("token"),
									templates.WithBinaryMapping("data"),
								),
								templates.WithDynamicProperties(false),
							),
							// templates.WithTemplateSettings(
							// 	templates.WithLifecycle(ilmPolicy, writeAlias),
							// ),
						),
					),
				),
				// Sessions index template.
				withIndexTemplateMigration(
					templates.NewIndexTemplate(
						indexTemplateName,
						templates.WithComponentTemplates(componentTemplateName),
						templates.WithIndexPatterns(indexPattern),
						templates.WithIndexTemplateMetadata(defaultMetadata),
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
	componentTemplates []*templates.ComponentTemplate
	indexTemplate      *templates.IndexTemplate
	ilmPolicy          *ilm.ILMPolicy
}

type templateMigrationOption Option[*templatesMigration]

func withComponentTemplatesMigration(templates ...*templates.ComponentTemplate) templateMigrationOption {
	return func(m *templatesMigration) {
		m.componentTemplates = templates
	}
}

func withIndexTemplateMigration(template *templates.IndexTemplate) templateMigrationOption {
	return func(m *templatesMigration) {
		m.indexTemplate = template
	}
}

func withILMPolicyMigration(policy *ilm.ILMPolicy) templateMigrationOption {
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

func getStatusCode(err error) int {
	var esErr *types.ElasticsearchError
	if errors.As(err, &esErr) {
		return esErr.Status
	}
	return http.StatusInternalServerError
}
