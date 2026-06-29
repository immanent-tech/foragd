// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/providers/elastic/ilm"
	"github.com/immanent-tech/foragd/providers/elastic/templates"
)

var (
	feedItemsCommonMappings = withComponentTemplatesMigration(
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
									Analyzer: &englishExactAnalyzer.Name,
								},
								"search": types.NewSearchAsYouTypeProperty(),
							},
						}),
						templates.WithTextMapping("description", &types.TextProperty{
							Type:     "text",
							Analyzer: &htmlAnalyzer.Name,
							// Fields: map[string]types.Property{
							// 	"semantic": types.NewSemanticTextProperty(),
							// },
						}),
						templates.WithTextMapping("authors", &types.TextProperty{
							Type:     "text",
							Analyzer: &personAnalyzer.Name,
							Fields: map[string]types.Property{
								"raw": types.NewKeywordProperty(),
								"exact": types.TextProperty{
									Analyzer: &englishExactAnalyzer.Name,
								},
								"search": types.NewSearchAsYouTypeProperty(),
							},
						}),
						templates.WithTextMapping("contributors", &types.TextProperty{
							Type:     "text",
							Analyzer: &personAnalyzer.Name,
							Fields: map[string]types.Property{
								"raw": types.NewKeywordProperty(),
								"exact": types.TextProperty{
									Analyzer: &englishExactAnalyzer.Name,
								},
								"search": types.NewSearchAsYouTypeProperty(),
							},
						}),
						templates.WithTextMapping("categories", &types.TextProperty{
							Type: "text",
							Fields: map[string]types.Property{
								"raw": types.NewKeywordProperty(),
								"raw_nocase": types.KeywordProperty{
									Normalizer: &lowercaseNormalizer.Name,
								},
								"exact": types.TextProperty{
									Analyzer: &englishExactAnalyzer.Name,
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
							englishExactAnalyzer.Name: englishExactAnalyzer.Definition,
							htmlAnalyzer.Name:         htmlAnalyzer.Definition,
							personAnalyzer.Name:       personAnalyzer.Definition,
						},
						Tokenizer: map[string]types.Tokenizer{
							emailTokenizer.Name: emailTokenizer.Definition,
						},
						Normalizer: map[string]types.Normalizer{
							lowercaseNormalizer.Name: lowercaseNormalizer.Definition,
						},
					}),
				),
			),
			templates.WithComponentTemplateMetadata(defaultMetadata),
		),
	)

	noDynamicMappingComponentTemplate = withComponentTemplatesMigration(
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
									Analyzer: &englishExactAnalyzer.Name,
								},
								"search": types.NewSearchAsYouTypeProperty(),
							},
						}),
						templates.WithTextMapping("content", &types.TextProperty{
							Type:     "text",
							Analyzer: new("html"),
							// Fields: map[string]types.Property{
							// 	"semantic": types.NewSemanticTextProperty(),
							// },
						}),
						templates.WithKeywordMapping("extension_type"),
						templates.WithFlattenedMapping("extension_data"),
					),
				),
				templates.WithTemplateSettings(
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
						templates.WithTextMapping("domain", &types.TextProperty{
							Type:     "text",
							Analyzer: &domainNameAnalyzer.Name,
							Fields: map[string]types.Property{
								"raw":    types.NewKeywordProperty(),
								"search": types.NewSearchAsYouTypeProperty(),
							},
						}),
					),
				),
				templates.WithTemplateSettings(
					templates.WithAnalysis(types.IndexSettingsAnalysis{
						Analyzer: map[string]types.Analyzer{
							domainNameAnalyzer.Name: domainNameAnalyzer.Definition,
						},
						Tokenizer: map[string]types.Tokenizer{
							domainTokenizer.Name: domainTokenizer.Definition,
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
						templates.WithKeywordMapping("subscription_type"),
						templates.WithFlattenedMapping("subscription"),
						templates.WithObjectMapping("metadata",
							templates.WithBooleanMapping("blocked"),
							templates.WithBooleanMapping("email_verified"),
							templates.WithBooleanMapping("promotional_email"),
							templates.WithBooleanMapping("policies_accepted"),
							templates.WithBooleanMapping("pending_deletion"),
							templates.WithObjectMapping("subscription_limit",
								templates.WithBooleanMapping("exceeded"),
								templates.WithDatetimeMapping("timestamp"),
							),
							templates.WithObjectMapping("newsletter_limit",
								templates.WithBooleanMapping("exceeded"),
								templates.WithDatetimeMapping("timestamp"),
							),
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
										Analyzer: &englishExactAnalyzer.Name,
									},
									"search": types.NewSearchAsYouTypeProperty(),
								},
							}),
							templates.WithTextMapping("categories", &types.TextProperty{
								Type: "text",
								Fields: map[string]types.Property{
									"raw": types.NewKeywordProperty(),
									"exact": types.TextProperty{
										Analyzer: &englishExactAnalyzer.Name,
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
	// englishExactAnalyzer provides exact (case-sensitive) matching.
	englishExactAnalyzer = templates.CustomAnalyser{
		Name: "english_exact",
		Definition: types.CustomAnalyzer{
			Tokenizer: "standard",
			Filter:    []string{"lowercase"},
		},
	}

	// emailTokenizer tokenizes text including emails and URLs.
	emailTokenizer = templates.CustomTokenizer{
		Name:       "email",
		Definition: types.NewUaxEmailUrlTokenizer(),
	}

	// personAnalyzer recognizes email addresses for people.
	personAnalyzer = templates.CustomAnalyser{
		Name: "html",
		Definition: types.CustomAnalyzer{
			Type:      "standard",
			Tokenizer: emailTokenizer.Name,
			Filter:    []string{"lowercase", "stop"},
		},
	}

	// htmlAnalyzer is the standard analyzer with HTML stripped/cleaned.
	htmlAnalyzer = templates.CustomAnalyser{
		Name: "html",
		Definition: types.CustomAnalyzer{
			Type:       "standard",
			Tokenizer:  emailTokenizer.Name,
			Filter:     []string{"lowercase", "stop"},
			CharFilter: []string{"html_strip"},
		},
	}

	// domainTokenizer tokenizes (splits) domain names.
	domainTokenizer = templates.CustomTokenizer{
		Name: "domain",
		Definition: types.CharGroupTokenizer{
			TokenizeOnChars: []string{"."},
		},
	}

	// domainNameAnalyzer is an analyzer for domain names.
	domainNameAnalyzer = templates.CustomAnalyser{
		Name: "domain",
		Definition: types.CustomAnalyzer{
			Type:      "standard",
			Tokenizer: domainTokenizer.Name,
			Filter:    []string{"lowercase"},
		},
	}

	// lowercaseNormalizer converts keywords to lowercase.
	lowercaseNormalizer = templates.CustomNormalizer{
		Name: "keyword_lowercase",
		Definition: types.CustomNormalizer{
			Type:   "keyword",
			Filter: []string{"lowercase"},
		},
	}

	// defaultMetadata defines default metadata.
	defaultMetadata = types.Metadata{
		"version":    json.RawMessage(fmt.Sprintf("%q", config.GetVersion())),
		"created_at": json.RawMessage(fmt.Sprintf("%q", time.Now().UTC().String())),
	}
)
