// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"maps"

	"github.com/elastic/go-elasticsearch/v9/typedapi/ilm/putlifecycle"
	"github.com/elastic/go-elasticsearch/v9/typedapi/ingest/putpipeline"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/dynamicmapping"
)

const (
	// FeedsSchemaPrefix is a prefix used for feed related index/mapping/settings.
	FeedsSchemaPrefix = "feeds"
	// ItemsSchemaPrefix is a prefix used for item related index/mapping/settings.
	ItemsSchemaPrefix = "feeditems"
	// UsersSchemaPrefix is a prefix used for user related index/mapping/settings.
	UsersSchemaPrefix = "users"
	// FavouritesSchemaPrefix is a prefix used for favourite related index/mapping/settings.
	FavouritesSchemaPrefix = "favourites"
	// SchedulerSchemaPrefix is a prefix used for scheduler related index/mapping/settings.
	SchedulerSchemaPrefix = "scheduler_jobs"
	// SessionsSchemaPrefix is a prefix used for sessions related index/mapping/settings.
	SessionsSchemaPrefix = "sessions"
	// SubscriptionsSchemaPrefix is a prefix used for subscriptions related index/mapping/settings.
	SubscriptionsSchemaPrefix = "subscriptions"

	ingestPipelineID = "gofeed"
)

var EnglishExactAnalyzerName = "english_exact"

var CommonObjectMappings = map[string]types.Property{
	"published":    types.NewDateNanosProperty(),
	"updated":      types.NewDateNanosProperty(),
	"title":        shortTextFieldProperty(),
	"description":  longTextFieldProperty(),
	"content":      longTextFieldProperty(),
	"authors":      shortTextFieldProperty(),
	"contributors": shortTextFieldProperty(),
	"categories":   shortTextFieldProperty(),
	"language":     shortTextFieldProperty(),
	"copyright":    longTextFieldProperty(),
	"source_type":  types.NewKeywordProperty(),
	"url":          types.NewKeywordProperty(),
	"image": types.ObjectProperty{
		Properties: map[string]types.Property{
			"value": types.NewKeywordProperty(),
			"title": longTextFieldProperty(),
		},
	},
}

// Option is a reusable generic function for applying options to a type.
type Option[T any] func(T) T

//
// SESSION
//

func sessionsMappings() map[string]types.Property {
	return map[string]types.Property{
		"expiry": types.NewDateNanosProperty(),
		"token":  types.NewKeywordProperty(),
		"data":   types.NewBinaryProperty(),
	}
}

func sessionsComponentTemplate() types.IndexState {
	return NewIndexState(
		WithMappings(&types.TypeMapping{
			Dynamic:    &dynamicmapping.False,
			Properties: sessionsMappings(),
		}),
		WithAliases(SessionsSchemaPrefix, types.Alias{}),
	)
}

//
// SCHEDULER
//

func schedulerJobsMappings() map[string]types.Property {
	return map[string]types.Property{
		"updated_at":   types.NewDateNanosProperty(),
		"job_options":  types.NewFlattenedProperty(),
		"job_data":     types.NewFlattenedProperty(),
		"job_type":     types.NewKeywordProperty(),
		"job_trigger":  types.NewFlattenedProperty(),
		"job_next_run": types.NewDateNanosProperty(),
	}
}

func schedulerJobsComponentTemplate() types.IndexState {
	return NewIndexState(
		WithMappings(&types.TypeMapping{
			Dynamic:    &dynamicmapping.False,
			Properties: schedulerJobsMappings(),
		}),
		WithAliases(SchedulerSchemaPrefix, types.Alias{}),
	)
}

//
// USERS
//

func userMappings() map[string]types.Property {
	return map[string]types.Property{
		"user_id":          types.NewKeywordProperty(),
		"external_user_id": types.NewKeywordProperty(),
		"provider":         types.NewKeywordProperty(),
		"created_at":       types.NewDateNanosProperty(),
		"updated_at":       types.NewDateNanosProperty(),
		"max_history":      types.NewKeywordProperty(),
		"settings":         types.NewFlattenedProperty(),
		"subscriptions": types.ObjectProperty{
			Properties: map[string]types.Property{
				"feed_id":         types.NewKeywordProperty(),
				"subscription_id": types.NewKeywordProperty(),
			},
		},
	}
}

func userComponentTemplate() types.IndexState {
	return NewIndexState(
		WithMappings(&types.TypeMapping{
			Dynamic:    &dynamicmapping.False,
			Properties: userMappings(),
		}),
		WithAliases(UsersSchemaPrefix, types.Alias{}),
	)
}

//
// FAVOURITES
//

func favouritesMappings() map[string]types.Property {
	return map[string]types.Property{
		"created_at":   types.NewDateNanosProperty(),
		"user_id":      types.NewKeywordProperty(),
		"favourite_id": types.NewKeywordProperty(),
		"type":         types.NewKeywordProperty(),
		"data":         types.NewFlattenedProperty(),
	}
}

func favouritesTemplate() types.IndexState {
	return NewIndexState(
		WithMappings(&types.TypeMapping{
			Dynamic:    &dynamicmapping.False,
			Properties: favouritesMappings(),
		}),
		WithAliases(FavouritesSchemaPrefix, types.Alias{}),
		WithIndexSettings(
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

//
// SUBSCRIPTIONS
//

func subscriptionsMappings() map[string]types.Property {
	return map[string]types.Property{
		"user_id":         types.NewKeywordProperty(),
		"subscription_id": types.NewKeywordProperty(),
		"feed_id":         types.NewKeywordProperty(),
		"updated_at":      types.NewDateNanosProperty(),
		"created_at":      types.NewDateNanosProperty(),
		"customisation": types.ObjectProperty{
			Properties: map[string]types.Property{
				"categories": shortTextFieldProperty(),
				"title":      shortTextFieldProperty(),
			},
		},
		"state": types.ObjectProperty{
			Properties: map[string]types.Property{
				"read":       types.NewBooleanProperty(),
				"starred":    types.NewBooleanProperty(),
				"updated_at": types.NewDateNanosProperty(),
			},
		},
		"item_states": types.NewFlattenedProperty(),
	}
}

func subscriptionsTemplate() types.IndexState {
	return NewIndexState(
		WithMappings(&types.TypeMapping{
			Dynamic:    &dynamicmapping.False,
			Properties: subscriptionsMappings(),
		}),
		WithAliases(SubscriptionsSchemaPrefix, types.Alias{}),
		WithIndexSettings(
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

//
// FEEDS
//

func feedsMappings() map[string]types.Property {
	mapping := map[string]types.Property{
		"feed_id":    types.NewKeywordProperty(),
		"created_at": types.NewDateNanosProperty(),
		"updated_at": types.NewDateNanosProperty(),
		"source_url": types.NewKeywordProperty(),
	}
	maps.Copy(mapping, CommonObjectMappings)
	return mapping
}

func feedsComponentTemplate() types.IndexState {
	return NewIndexState(
		WithMappings(&types.TypeMapping{
			Dynamic:    &dynamicmapping.False,
			Properties: feedsMappings(),
		}),
		WithAliases(FeedsSchemaPrefix, types.Alias{}),
		WithIndexSettings(
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

//
// FEED ITEMS
//

func itemsMappings() map[string]types.Property {
	mapping := map[string]types.Property{
		"@timestamp": types.NewDateNanosProperty(),
		"feed_id":    types.NewKeywordProperty(),
		"item_id":    types.NewKeywordProperty(),
		"created":    types.NewDateNanosProperty(),
	}
	maps.Copy(mapping, CommonObjectMappings)
	return mapping
}

func itemsComponentTemplate() types.IndexState {
	return NewIndexState(
		WithMappings(&types.TypeMapping{
			Dynamic:    &dynamicmapping.False,
			Properties: itemsMappings(),
		}),
		WithAliases(ItemsSchemaPrefix, types.Alias{}),
		WithIndexSettings(
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

// ingestPipelineFeeds is an ingest pipeline to clean-up feed and item data.
func ingestPipelineFeeds() *putpipeline.Request {
	return NewIngestPipeline(
		WithRemoveProcessor(
			RemoveDescription("Remove deprecated and/or unneeded fields"),
			RemoveFields("author", "published", "updated", "items"),
			RemoveIgnoreMissing(true),
		),
	)
}

// shortTextFieldMapping defines a mapping appropriate for fields containing short amounts of text, such as titles and
// categories.
func shortTextFieldProperty() types.TextProperty {
	return types.TextProperty{
		Type: "text",
		Fields: map[string]types.Property{
			"raw": types.NewKeywordProperty(),
			"exact": types.TextProperty{
				Analyzer: &EnglishExactAnalyzerName,
			},
			"search": types.NewSearchAsYouTypeProperty(),
		},
	}
}

// longTextFieldMapping defines a mapping appropriate for fields containing longer amounts of text, such as
// descriptions, and full content.
func longTextFieldProperty() types.TextProperty {
	return types.TextProperty{
		Type: "text",
	}
}
