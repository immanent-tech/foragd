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
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/conflicts"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/providers/elastic/ilm"
	"github.com/immanent-tech/foragd/providers/elastic/reindex"
	"github.com/immanent-tech/foragd/providers/elastic/templates"
)

// Index constants.

const (
	feedsIndexPrefix          = "feeds"
	itemsSchemaPrefix         = "items"
	favoritesSchemaPrefix     = "favorites"
	usersSchemaPrefix         = "users"
	subscriptionsSchemaPrefix = "subscriptions"
	schedulerIndexPrefix      = "scheduler"
	sessionsSchemaPrefix      = "sessions"
	// indexWriteSuffix is the suffix appended to indicies that are used for write (indexing) operations.
	indexWriteSuffix = "_rw"
	// indexReadSuffix is the suffix appended to indicies that are used for read (search, get) operations.
	indexReadSuffix = "_ro"
)

func FeedsIndexRO() string {
	return feedsIndexPrefix + "_" + config.GetEnvironment().String() + indexReadSuffix
}

func FeedsIndexRW() string {
	return feedsIndexPrefix + "_" + config.GetEnvironment().String() + indexWriteSuffix
}

func ItemsIndexRO() string {
	return itemsSchemaPrefix + "_" + config.GetEnvironment().String() + indexReadSuffix
}

func ItemsIndexRW() string {
	return itemsSchemaPrefix + "_" + config.GetEnvironment().String() + indexWriteSuffix
}

func SubscriptionsIndexRO() string {
	return subscriptionsSchemaPrefix + "_" + config.GetEnvironment().String() + indexReadSuffix
}

func SubscriptionsIndexRW() string {
	return subscriptionsSchemaPrefix + "_" + config.GetEnvironment().String() + indexWriteSuffix
}

func FavoritesIndexRO() string {
	return favoritesSchemaPrefix + "_" + config.GetEnvironment().String() + indexReadSuffix
}

func FavoritesIndexRW() string {
	return favoritesSchemaPrefix + "_" + config.GetEnvironment().String() + indexWriteSuffix
}

func UsersIndexRO() string {
	return usersSchemaPrefix + "_" + config.GetEnvironment().String() + indexReadSuffix
}
func UsersIndexRW() string {
	return usersSchemaPrefix + "_" + config.GetEnvironment().String() + indexWriteSuffix
}

func SessionsIndexRO() string {
	return sessionsSchemaPrefix + "_" + config.GetEnvironment().String() + indexReadSuffix
}

func SessionsIndexRW() string {
	return sessionsSchemaPrefix + "_" + config.GetEnvironment().String() + indexWriteSuffix
}

func SchedulerIndexRO() string {
	return schedulerIndexPrefix + "_" + config.GetEnvironment().String() + indexReadSuffix
}

func SchedulerIndexRW() string {
	return schedulerIndexPrefix + "_" + config.GetEnvironment().String() + indexWriteSuffix
}

var feedItemsCommonMappings = withComponentTemplatesMigration(
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
								Analyzer: &englishExactAnalyzerName,
							},
							"search": types.NewSearchAsYouTypeProperty(),
						},
					}),
					templates.WithTextMapping("description", &types.TextProperty{
						Type: "text",
						// Fields: map[string]types.Property{
						// 	"semantic": types.NewSemanticTextProperty(),
						// },
					}),
					templates.WithTextMapping("authors", &types.TextProperty{
						Type: "text",
						Fields: map[string]types.Property{
							"raw": types.NewKeywordProperty(),
							"exact": types.TextProperty{
								Analyzer: &englishExactAnalyzerName,
							},
							"search": types.NewSearchAsYouTypeProperty(),
						},
					}),
					templates.WithTextMapping("contributors", &types.TextProperty{
						Type: "text",
						Fields: map[string]types.Property{
							"raw": types.NewKeywordProperty(),
							"exact": types.TextProperty{
								Analyzer: &englishExactAnalyzerName,
							},
							"search": types.NewSearchAsYouTypeProperty(),
						},
					}),
					templates.WithTextMapping("categories", &types.TextProperty{
						Type: "text",
						Fields: map[string]types.Property{
							"raw": types.NewKeywordProperty(),
							"raw_nocase": types.KeywordProperty{
								Normalizer: &lowercaseNormalizerName,
							},
							"exact": types.TextProperty{
								Analyzer: &englishExactAnalyzerName,
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
						englishExactAnalyzerName: types.CustomAnalyzer{
							Tokenizer: "standard",
							Filter:    []string{"lowercase"},
						},
					},
					Normalizer: map[string]types.Normalizer{
						lowercaseNormalizerName: types.CustomNormalizer{
							Type:   "keyword",
							Filter: []string{"lowercase"},
						},
					},
				}),
			),
		),
		templates.WithComponentTemplateMetadata(defaultMetadata),
	),
)

var noDynamicMappingComponentTemplate = withComponentTemplatesMigration(
	templates.NewComponentTemplate(
		"no_dynamic_mapping",
		templates.NewTemplate(
			templates.WithTemplateMapping(
				templates.WithDynamicProperties("strict"),
			),
		),
		templates.WithComponentTemplateMetadata(defaultMetadata),
	),
)

var (
	// itemsComponentTemplate contains the field mappings for Items.
	itemsComponentTemplate = withComponentTemplatesMigration(
		templates.NewComponentTemplate(
			"items_component_template",
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
									Analyzer: &englishExactAnalyzerName,
								},
								"search": types.NewSearchAsYouTypeProperty(),
							},
						}),
						templates.WithTextMapping("content", &types.TextProperty{
							Type: "text",
							// Fields: map[string]types.Property{
							// 	"semantic": types.NewSemanticTextProperty(),
							// },
						}),
						templates.WithKeywordMapping("extension_type"),
						templates.WithFlattenedMapping("extension_data"),
					),
				),
				templates.WithTemplateSettings(
					templates.WithAnalysis(types.IndexSettingsAnalysis{
						Analyzer: map[string]types.Analyzer{
							englishExactAnalyzerName: types.CustomAnalyzer{
								Tokenizer: "standard",
								Filter:    []string{"lowercase"},
							},
						},
					}),
					templates.WithLifecycle("items_ilm_policy", ItemsIndexRW()),
				),
			),
		),
	)
	// itemsIndexTemplate contains the settings for Items indices.
	itemsIndexTemplate = withIndexTemplateMigration(
		templates.NewIndexTemplate(
			"items_index_template",
			templates.WithComponentTemplates("feed_items_common", "items_component_template"),
			templates.WithIndexPatterns(itemsSchemaPrefix+"-*"),
			templates.WithIndexTemplateMetadata(defaultMetadata),
		),
	)
	// itemsILMPolicy is the ILM policy for Items indices.
	itemsILMPolicy = withILMPolicyMigration(
		ilm.NewILMPolicy(
			"items_ilm_policy",
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
	)
)

var (
	// favoriteItemsComponentTemplate contains the field mappings for Favorites indices.
	favoriteItemsComponentTemplate = withComponentTemplatesMigration(
		templates.NewComponentTemplate(
			"favorites_component_template",
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
				),
				templates.WithTemplateSettings(
					templates.WithAnalysis(types.IndexSettingsAnalysis{
						Analyzer: map[string]types.Analyzer{
							englishExactAnalyzerName: types.CustomAnalyzer{
								Tokenizer: "standard",
								Filter:    []string{"lowercase"},
							},
						},
					}),
				),
			),
			templates.WithComponentTemplateMetadata(defaultMetadata),
		),
	)
	// favoriteItemsIndexTemplate contains the settings for Favorites indices.
	favoriteItemsIndexTemplate = withIndexTemplateMigration(
		templates.NewIndexTemplate(
			"favorites_index_template",
			templates.WithComponentTemplates("feed_items_common", "favorites_component_template"),
			templates.WithIndexPatterns(favoritesSchemaPrefix+"-*"),
			templates.WithIndexTemplateMetadata(defaultMetadata),
		),
	)
)

var (
	// feedsComponentTemplate contains the field mappings for Feeds.
	feedsComponentTemplate = withComponentTemplatesMigration(
		templates.NewComponentTemplate(
			"feeds_component_template",
			templates.NewTemplate(
				templates.WithTemplateMapping(
					templates.WithProperties(
						templates.WithKeywordMapping("feed_id"),
						templates.WithDatetimeMapping("created_at"),
						templates.WithDatetimeMapping("last_fetched"),
						templates.WithInt64Mapping("update_interval"),
						templates.WithKeywordMapping("source_urls"),
						templates.WithKeywordMapping("fetch_method"),
						templates.WithFlattenedMapping("quirks"),
					),
				),
				templates.WithTemplateSettings(
					templates.WithAnalysis(types.IndexSettingsAnalysis{
						Analyzer: map[string]types.Analyzer{
							englishExactAnalyzerName: types.CustomAnalyzer{
								Tokenizer: "standard",
								Filter:    []string{"lowercase"},
							},
						},
					}),
				),
			),
			templates.WithComponentTemplateMetadata(defaultMetadata),
		),
	)
	// feedsIndexTemplate contains the settings for Feeds indices.
	feedsIndexTemplate = withIndexTemplateMigration(
		templates.NewIndexTemplate(
			"feeds_index_template",
			templates.WithComponentTemplates("feed_items_common", "feeds_component_template"),
			templates.WithIndexPatterns(feedsIndexPrefix+"-*"),
			templates.WithIndexTemplateMetadata(defaultMetadata),
		),
	)
)

var (
	// usersComponentTemplate contains the field mappings for Users indicies.
	usersComponentTemplate = withComponentTemplatesMigration(
		templates.NewComponentTemplate(
			"users_component_template",
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
						templates.WithDatetimeMapping("last_login"),
						templates.WithInt64Mapping("login_count"),
						templates.WithKeywordMapping("max_history"),
						templates.WithFlattenedMapping("settings"),
						templates.WithObjectMapping("metadata",
							templates.WithBooleanMapping("blocked"),
							templates.WithBooleanMapping("email_verified"),
							templates.WithBooleanMapping("promotional_email"),
							templates.WithBooleanMapping("policies_accepted"),
							templates.WithBooleanMapping("pending_deletion"),
						),
						templates.WithKeywordMapping("item_favorites"),
					),
				),
			),
		),
	)
	// usersIndexTemplate contains the settings for Users indices.
	usersIndexTemplate = withIndexTemplateMigration(
		templates.NewIndexTemplate(
			"users_index_template",
			templates.WithComponentTemplates("users_component_template"),
			templates.WithIndexPatterns(usersSchemaPrefix+"-*"),
			templates.WithIndexTemplateMetadata(defaultMetadata),
		),
	)
)

var (
	// subscriptionsComponentTemplate contains the field mappings for Subscriptions indices.
	subscriptionsComponentTemplate = withComponentTemplatesMigration(
		templates.NewComponentTemplate(
			"subscriptions_component_template",
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
										Analyzer: &englishExactAnalyzerName,
									},
									"search": types.NewSearchAsYouTypeProperty(),
								},
							}),
							templates.WithTextMapping("categories", &types.TextProperty{
								Type: "text",
								Fields: map[string]types.Property{
									"raw": types.NewKeywordProperty(),
									"exact": types.TextProperty{
										Analyzer: &englishExactAnalyzerName,
									},
									"search": types.NewSearchAsYouTypeProperty(),
								},
							}),
							templates.WithKeywordMapping("image_url"),
						),
						templates.WithFlattenedMapping("settings"),
						templates.WithObjectMapping("feed_data",
							templates.WithKeywordMapping("feed_id"),
							templates.WithFlattenedMapping("article_states"),
							templates.WithObjectMapping("article_filters",
								templates.WithKeywordMapping("text"),
								templates.WithKeywordMapping("categories"),
								templates.WithKeywordMapping("authors"),
							),
						),
						templates.WithObjectMapping("group_data",
							templates.WithKeywordMapping("subscriptions"),
							templates.WithObjectMapping("article_filters",
								templates.WithKeywordMapping("text"),
								templates.WithKeywordMapping("categories"),
								templates.WithKeywordMapping("authors"),
							),
						),
						templates.WithObjectMapping("email_data",
							templates.WithKeywordMapping("feed_id"),
							templates.WithKeywordMapping("email_sender_id"),
							templates.WithFlattenedMapping("article_states"),
							templates.WithObjectMapping("article_filters",
								templates.WithKeywordMapping("text"),
								templates.WithKeywordMapping("categories"),
								templates.WithKeywordMapping("authors"),
							),
						),
						templates.WithFlattenedMapping("search_data"),
						templates.WithObjectMapping("email_data",
							templates.WithKeywordMapping("sender"),
							templates.WithKeywordMapping("feed_id"),
							templates.WithFlattenedMapping("article_states"),
							templates.WithObjectMapping("article_filters",
								templates.WithKeywordMapping("text"),
								templates.WithKeywordMapping("categories"),
								templates.WithKeywordMapping("authors"),
							),
						),
					),
				),
				templates.WithTemplateSettings(
					templates.WithAnalysis(types.IndexSettingsAnalysis{
						Analyzer: map[string]types.Analyzer{
							englishExactAnalyzerName: types.CustomAnalyzer{
								Tokenizer: "standard",
								Filter:    []string{"lowercase"},
							},
						},
					}),
				),
			),
		),
	)
	// subscriptionsIndexTemplate contains the settings for Subscriptions indices.
	subscriptionsIndexTemplate = withIndexTemplateMigration(
		templates.NewIndexTemplate(
			"subscriptions_index_template",
			templates.WithComponentTemplates("subscriptions_component_template"),
			templates.WithIndexPatterns(subscriptionsSchemaPrefix+"-*"),
			templates.WithIndexTemplateMetadata(defaultMetadata),
		),
	)
)

var (
	// schedulerComponentTemplate contains the field mappings for Scheduler indices.
	schedulerComponentTemplate = withComponentTemplatesMigration(
		templates.NewComponentTemplate(
			"scheduler_component_template",
			templates.NewTemplate(
				templates.WithTemplateMapping(
					templates.WithProperties(
						templates.WithDatetimeMapping("created_at"),
						templates.WithDatetimeMapping("updated_at"),
						templates.WithFlattenedMapping("job_options"),
						templates.WithFlattenedMapping("job_data"),
						templates.WithKeywordMapping("job_type"),
						templates.WithKeywordMapping("job_key"),
						templates.WithTextMapping("job_description", &types.TextProperty{
							Type: "text",
							Fields: map[string]types.Property{
								"raw": types.NewKeywordProperty(),
							},
						}),
						templates.WithKeywordMapping("job_trigger_type"),
						templates.WithFlattenedMapping("job_trigger"),
						templates.WithDatetimeMapping("job_next_run"),
					),
				),
			),
		),
	)
	// schedulerIndexTemplate contains the settings for Scheduler indices.
	schedulerIndexTemplate = withIndexTemplateMigration(
		templates.NewIndexTemplate(
			"scheduler_index_template",
			templates.WithComponentTemplates("scheduler_component_template", "no_dynamic_mapping"),
			templates.WithIndexPatterns(schedulerIndexPrefix+"-*"),
			templates.WithIndexTemplateMetadata(defaultMetadata),
		),
	)
)

var (
	sessionsComponentTemplate = withComponentTemplatesMigration(
		templates.NewComponentTemplate(
			"sessions_component_template",
			templates.NewTemplate(
				templates.WithTemplateMapping(
					templates.WithProperties(
						templates.WithDatetimeMapping("updated_at"),
						templates.WithDatetimeMapping("expiry"),
						templates.WithKeywordMapping("token"),
						templates.WithBinaryMapping("data"),
					),
				),
			),
		),
	)
	sessionsIndexTemplate = withIndexTemplateMigration(
		templates.NewIndexTemplate(
			"sessions_index_template",
			templates.WithComponentTemplates("sessions_component_template"),
			templates.WithIndexPatterns(sessionsSchemaPrefix+"-*"),
			templates.WithIndexTemplateMetadata(defaultMetadata),
		),
	)
)

var (
	// logsILMPolicy is a general-purpose ILM policy for logs indices. Indices are quickly moved through the phases from
	// hot to cold and then kept indefinitely in the cold phase.
	logsILMPolicy = ilm.NewILMPolicy(
		"logs_ilm_policy",
		ilm.WithPhase("hot",
			ilm.WithMinAge("0ms"),
			ilm.WithActions(
				ilm.WithRolloverMaxAge("30d"),
				ilm.WithRolloverMaxSize("50gb"),
			),
		),
		ilm.WithPhase("warm",
			ilm.WithMinAge("2d"),
			ilm.WithActions(
				ilm.WithShrinkToShards(1),
				ilm.WithAllowWriteAfterShrink(false),
				ilm.WithForceMergeSegments(1),
			),
		),
		ilm.WithPhase("cold",
			ilm.WithMinAge("30d"),
		),
	)
)

var (
	englishExactAnalyzerName = "english_exact"
	lowercaseNormalizerName  = "keyword_lowercase"
	// defaultMetadata defines default metadata.
	defaultMetadata = types.Metadata{
		"version":    json.RawMessage(fmt.Sprintf("%q", config.GetVersion())),
		"created_at": json.RawMessage(fmt.Sprintf("%q", time.Now().UTC().String())),
	}
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
func CreateIndices(ctx context.Context, api *elasticsearch.TypedClient, opts *IndicesOptions) error {
	// If no indices are specified, create indices for all items.
	if slices.Contains(opts.Indices, "all") {
		opts.Indices = allIndices
	}
	for prefix := range slices.Values(opts.Indices) {
		index := generateIndexName(prefix)
		writeAlias := prefix + "_" + config.GetEnvironment().String() + indexWriteSuffix
		readAlias := prefix + "_" + config.GetEnvironment().String() + indexReadSuffix
		// Create a scheduler index if one doesn't exist.
		if _, err := createIndexIfNotExists(ctx, api, prefix); err != nil {
			return fmt.Errorf("create index: %w", err)
		}
		// Add appropriate aliases.
		if err := updateAlias(ctx, api, readAlias, index); err != nil {
			return fmt.Errorf("add read alias: %w", err)
		}
		if err := updateAlias(ctx, api, writeAlias, index); err != nil {
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
	index := generateIndexName(prefix)
	writeAlias := prefix + "_" + config.GetEnvironment().String() + indexWriteSuffix
	readAlias := prefix + "_" + config.GetEnvironment().String() + indexReadSuffix

	// Create index.
	if _, err := createIndexIfNotExists(ctx, api, prefix); err != nil {
		return fmt.Errorf("could not create index %s: %w", index, err)
	}

	// Update the write alias.
	if err := updateAlias(ctx, api, writeAlias, index); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	// Reindex if requested.
	if found, err := api.Indices.Exists(readAlias).Do(ctx); err != nil || !found {
		return fmt.Errorf("could not determine %s index state: %w", readAlias, err)
	}
	reindexResp, err := reindex.NewReindexOperation(api, reindex.NewSource(readAlias), reindex.NewDest(index, "")).
		WaitForCompletion(true).
		Conflicts(conflicts.Proceed).
		RequestsPerSecond("500").
		Do(ctx)
	const statusCodeErrLevel = 500
	switch {
	case err != nil:
		if getStatusCode(err) >= statusCodeErrLevel {
			return fmt.Errorf("could not reindex: %w", err)
		}
		slogctx.FromCtx(ctx).Info("Reindex completed with warnings.",
			slog.String("src", readAlias),
			slog.String("dest", index),
			slog.Int64("took", *reindexResp.Took),
			slog.Any("warnings", err),
		)
	default:
		slogctx.FromCtx(ctx).Info("Reindex completed.",
			slog.String("src", readAlias),
			slog.String("dest", index),
			slog.Int64("took", *reindexResp.Took),
		)
	}
	// Update the read alias.
	if err = updateAlias(ctx, api, readAlias, index); err != nil {
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
		if _, found := aliases.Aliases[alias]; found {
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

func createIndexIfNotExists(ctx context.Context, api *elasticsearch.TypedClient, prefix string) (bool, error) {
	index := generateIndexName(prefix)
	// Create index.
	found, err := api.Indices.Exists(index).Do(ctx)
	if err != nil {
		return found, fmt.Errorf("could not determine %s index state: %w", index, err)
	}
	if !found {
		_, err = api.Indices.Create(index).Do(ctx)
		if err != nil {
			return found, fmt.Errorf("could not create index %s: %w", index, err)
		}
		slogctx.FromCtx(ctx).Info("New index created.",
			slog.String("name", index),
		)
	}
	slogctx.FromCtx(ctx).Info("Index already exists.",
		slog.String("name", index),
	)

	return found, nil
}

func generateIndexName(prefix string) string {
	return strings.Join(
		[]string{prefix, config.GetEnvironment().String(), time.Now().Format("20060102150405"), "000000"},
		"-",
	)
}

func getStatusCode(err error) int {
	if esErr, ok := errors.AsType[*types.ElasticsearchError](err); ok {
		return esErr.Status
	}
	return http.StatusInternalServerError
}
