// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v9/typedapi/ilm/putlifecycle"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"

	"github.com/immanent-tech/foragd/config"
)

const (
	// FeedsSchemaPrefix is a prefix used for feed related index/mapping/settings.
	FeedsSchemaPrefix = "feeds"
	// ItemsSchemaPrefix is a prefix used for item related index/mapping/settings.
	ItemsSchemaPrefix = "items"
	// ArticleArchiveSchemaPrefix is a prefix used for item archive related index/mapping/settings.
	ArticleArchiveSchemaPrefix = "article_archive"
	// UsersSchemaPrefix is a prefix used for user related index/mapping/settings.
	UsersSchemaPrefix = "users"
	// SchedulerJobsPrefix is a prefix used for scheduler related index/mapping/settings.
	SchedulerJobsPrefix = "scheduler_jobs"
	// SchedulerStatePrefix is a prefix used for scheduler state related index/mapping/settings.
	SchedulerStatePrefix = "scheduler_state"
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
	FeedItemCommonMappings = NewProperties(
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
		),
	)
	// defaultMetadata defines default metadata.
	defaultMetadata = types.Metadata{
		"version":    json.RawMessage(fmt.Sprintf("%q", config.Version)),
		"created_at": json.RawMessage(fmt.Sprintf("%q", time.Now().UTC().String())),
	}
)

// Option is a reusable generic function for applying options to a type.
type Option[T any] func(T)

// sessionsComponentTemplate is the template for sessions indices.
func sessionsComponentTemplate() *Template {
	return NewTemplate(
		WithTemplateMapping(
			WithProperties(
				WithDatetimeMapping("expiry"),
				WithKeywordMapping("token"),
				WithBinaryMapping("data"),
			),
			WithDynamicProperties(false),
		),
	)
}

// schedulerJobsComponentTemplate is the template for scheduler jobs indices.
func schedulerJobsComponentTemplate() *Template {
	return NewTemplate(
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
	)
}

// schedulerStateComponentTemplate is the template for scheduler state indicies.
func schedulerStateComponentTemplate() *Template {
	return NewTemplate(
		WithTemplateMapping(
			WithProperties(
				WithDatetimeMapping("updated_at"),
				WithFlattenedMapping("job_data"),
			),
			WithDynamicProperties(false),
		),
	)
}

// usersComponentTemplate is the template for users indices.
func userComponentTemplate() *Template {
	return NewTemplate(
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
	)
}

// feedsComponentTemplate is the template for feeds indices.
func feedsComponentTemplate() *Template {
	return NewTemplate(
		WithTemplateMapping(
			WithProperties(
				WithKeywordMapping("feed_id"),
				WithDatetimeMapping("created_at"),
				WithDatetimeMapping("last_fetched"),
				WithKeywordMapping("source_urls"),
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
		),
	)
}

// articleArchiveComponentTemplate is the template for article archive indices.
func articleArchiveComponentTemplate() *Template {
	return NewTemplate(
		WithTemplateMapping(
			WithProperties(
				WithDatetimeMapping("@timestamp"),
				WithDatetimeMapping("created"),
				WithKeywordMapping("feed_id"),
				WithKeywordMapping("item_id"),
				WithKeywordMapping("user_id"),
				WithKeywordMapping("subscription_id"),
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
		),
	)
}

// itemsComponentTemplate is the template for items indices.
func itemsComponentTemplate() *Template {
	return NewTemplate(
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
			WithLifecycle(ItemsSchemaPrefix),
		),
	)
}

// logsComponentTemplate is the template for logs indices.
func logsComponentTemplate() *Template {
	return NewTemplate(
		WithTemplateMapping(
			WithDynamicProperties(true),
		),
		WithAlias(LogsSchemaPrefix, nil),
		WithTemplateSettings(
			WithMode("logsdb"),
			WithLifecycle(LogsSchemaPrefix),
		),
	)
}

// itemsILMPolicy is the ILM policy for feed items indices.
func itemsILMPolicy() *putlifecycle.Request {
	return defaultILMPolicy()
}

// defaultILMPolicy is a default ILM policy that will apply the following phases:
//
// - Hot Phase: rollover indices once they reach 50gb size.
//
// - Warm Phase: shrink and forcemerge indices down to 1 shard and 1 segment.
//
// - Delete Phase: delete indices older than 735 days.
func defaultILMPolicy() *putlifecycle.Request {
	return NewILMPolicy(
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
	)
}
