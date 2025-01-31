// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"github.com/elastic/go-elasticsearch/v8/typedapi/cluster/putcomponenttemplate"
	"github.com/elastic/go-elasticsearch/v8/typedapi/ilm/putlifecycle"
	"github.com/elastic/go-elasticsearch/v8/typedapi/indices/putindextemplate"
	"github.com/elastic/go-elasticsearch/v8/typedapi/ingest/putpipeline"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

const (
	MappingsSuffix        = "_mappings"
	SettingsSuffix        = "_settings"
	FeedsSchemaPrefix     = "feeds"
	FeedItemsSchemaPrefix = "feeditems"
	UsersSchemaPrefix     = "users"
	SchedulerJobsPrefix   = "scheduler_jobs"
	SessionsPrefix        = "sessions"

	IngestPipelineID = "gofeed"

	FeedsMappings         = FeedsSchemaPrefix + MappingsSuffix
	FeedsSettings         = FeedsSchemaPrefix + SettingsSuffix
	FeedsItemsMappings    = FeedItemsSchemaPrefix + MappingsSuffix
	FeedsItemsSettings    = FeedItemsSchemaPrefix + SettingsSuffix
	UsersMappings         = UsersSchemaPrefix + MappingsSuffix
	UsersSettings         = UsersSchemaPrefix + SettingsSuffix
	SchedulerJobsMappings = SchedulerJobsPrefix + MappingsSuffix
	SchedulerJobsSettings = SchedulerJobsPrefix + SettingsSuffix
	SessionsMappings      = SessionsPrefix + MappingsSuffix
	SessionsSettings      = SessionsPrefix + SettingsSuffix

	schemaVersion = "v0.0.0"
)

var defaultMetadata = NewMetadata(WithMetadataField("version", schemaVersion))

// Option is a reusable generic function for applying options to a type.
type Option[T any] func(T) T

//
// SESSION
//

// SessionsMappingsTemplate returns a Component Template for
// sessions index field mappings.
func SessionsMappingsTemplate() *putcomponenttemplate.Request {
	return NewComponentTemplateRequest(
		WithIndexOptions(
			NewIndexState(
				WithMappings(
					NewPropertyMapping(
						WithoutDynamicMapping(),
						WithDateNanosProperty("expiry"),
						WithKeywordProperty("token"),
						WithBinaryProperty("data"),
					),
				),
			),
		),
	)
}

// SessionsSettingsTemplate returns a Component Template for sessions
// index settings.
func SessionsSettingsTemplate() *putcomponenttemplate.Request {
	return NewComponentTemplateRequest(
		WithIndexOptions(
			NewIndexState(
				WithAliases(SessionsPrefix, types.Alias{}),
			),
		),
	)
}

// SessionsIndexTemplate returns an Index Template for sessions indices.
func SessionsIndexTemplate() *putindextemplate.Request {
	return NewIndexTemplateRequest(
		WithIndexPatterns(SessionsPrefix+"_*"),
		WithComponentTemplates(SessionsMappings, SessionsSettings),
		WithPriority(500),
	)
}

//
// SCHEDULER
//

// SchedulerJobsMappingsTemplate returns a Component Template for
// scheduler jobs index field mappings.
func SchedulerJobsMappingsTemplate() *putcomponenttemplate.Request {
	return NewComponentTemplateRequest(
		WithIndexOptions(
			NewIndexState(
				WithMappings(
					NewPropertyMapping(
						WithoutDynamicMapping(),
						WithDateNanosProperty("created_at"),
						WithFlattenedProperty("job_options"),
						WithFlattenedProperty("job_data"),
						WithKeywordProperty("job_trigger"),
						WithDateNanosProperty("job_next_run"),
					),
				),
			),
		),
	)
}

// ComponentTemplateFeedItemsSettings returns a Component Template for scheduler
// jobs index settings.
func SchedulerJobsSettingsTemplate() *putcomponenttemplate.Request {
	return NewComponentTemplateRequest(
		WithIndexOptions(
			NewIndexState(
				WithAliases(SchedulerJobsPrefix, types.Alias{}),
			),
		),
	)
}

// SchedulerJobsIndexTemplate returns an Index Template for scheduler jobs indices.
func SchedulerJobsIndexTemplate() *putindextemplate.Request {
	return NewIndexTemplateRequest(
		WithIndexPatterns(SchedulerJobsPrefix+"_*"),
		WithComponentTemplates(SchedulerJobsMappings, SchedulerJobsSettings),
		WithPriority(500),
	)
}

//
// USERS
//

// UserMappingsTemplate returns a Component Template for users
// index field mappings.
func UserMappingsTemplate() *putcomponenttemplate.Request {
	return NewComponentTemplateRequest(
		WithIndexOptions(
			NewIndexState(
				WithMappings(
					NewPropertyMapping(
						WithoutDynamicMapping(),
						WithKeywordProperty("user_id"),
						WithDateNanosProperty("created_at"),
						WithDateNanosProperty("updated_at"),
						WithObjectProperty("subscriptions", map[string]types.Property{
							"categories":  asTextAndKeyword(),
							"name":        asTextAndKeyword(),
							"created_at":  types.NewDateNanosProperty(),
							"updated_at":  types.NewDateNanosProperty(),
							"marked_read": types.NewDateNanosProperty(),
						}),
						WithObjectProperty("feed_item_states", map[string]types.Property{
							"item_states": types.ObjectProperty{
								Properties: map[string]types.Property{
									"state":      types.NewKeywordProperty(),
									"updated_at": types.NewDateNanosProperty(),
								},
							},
						}),
					),
				),
			),
		),
	)
}

// UsersSettingsTemplate returns a Component Template for users
// index settings.
func UsersSettingsTemplate() *putcomponenttemplate.Request {
	return NewComponentTemplateRequest(
		WithIndexOptions(
			NewIndexState(
				WithAliases(UsersSchemaPrefix, types.Alias{}),
			),
		),
	)
}

// UsersIndexTemplate returns an Index Template for users indices.
func UsersIndexTemplate() *putindextemplate.Request {
	return NewIndexTemplateRequest(
		WithIndexPatterns(UsersSchemaPrefix+"_*"),
		WithComponentTemplates(UsersMappings, UsersSettings),
		WithPriority(500),
	)
}

//
// FEEDS
//

// FeedsMappingsTemplate returns a Component Template for feeds
// index field mappings.
func FeedsMappingsTemplate() *putcomponenttemplate.Request {
	return NewComponentTemplateRequest(
		WithIndexOptions(
			NewIndexState(
				WithMappings(
					NewPropertyMapping(
						WithoutDynamicMapping(),
						WithKeywordProperty("feed_id"),
						WithDateNanosProperty("created_at"),
						WithDateNanosProperty("updated_at"),
						WithTextAndKeywordProperty("title"),
						WithTextProperty("description"),
						WithTextProperty("content"),
						WithKeywordProperty("link"),
						WithKeywordProperty("feedLink"),
						WithKeywordProperty("links"),
						WithKeywordProperty("feedType"),
						WithKeywordProperty("feedVersion"),
						WithDateNanosProperty("updatedParsed"),
						WithDateNanosProperty("publishedParsed"),
						WithObjectProperty("authors", map[string]types.Property{
							"name":  asTextAndKeyword(),
							"email": asTextAndKeyword(),
						}),
						WithTextAndKeywordProperty("language"),
						WithObjectProperty("image", map[string]types.Property{
							"url":   types.NewKeywordProperty(),
							"title": asTextAndKeyword(),
						}),
						WithTextAndKeywordProperty("copyright"),
						WithTextAndKeywordProperty("generator"),
						WithTextAndKeywordProperty("categories"),
						WithObjectProperty("dublincoreext", DublinCoreMappingsTemplate()),
						WithObjectProperty("itunesext", ItunesMappingTemplate()),
						WithFlattenedProperty("extensions"),
						WithFlattenedProperty("custom"),
					),
				),
			),
		),
	)
}

// FeedsSettingsTemplate returns a Component Template for feeds
// index settings.
func FeedsSettingsTemplate() *putcomponenttemplate.Request {
	return NewComponentTemplateRequest(
		WithIndexOptions(
			NewIndexState(
				WithAliases(FeedsSchemaPrefix, types.Alias{}),
			),
		),
	)
}

// FeedsIndexTemplate returns an Index Template for feeds indices.
func FeedsIndexTemplate() *putindextemplate.Request {
	return NewIndexTemplateRequest(
		WithIndexPatterns(FeedsSchemaPrefix+"_*"),
		WithComponentTemplates(FeedsMappings, FeedsSettings),
	)
}

//
// FEED ITEMS
//

// ComponentTemplateFeedsMappings returns a Component Template for feed items
// index field mappings.
func FeedItemsMappingsTemplate() *putcomponenttemplate.Request {
	return NewComponentTemplateRequest(
		WithIndexOptions(
			NewIndexState(
				WithMappings(
					NewPropertyMapping(
						WithoutDynamicMapping(),
						WithDateNanosProperty("@timestamp"),
						WithKeywordProperty("feed_id"),
						WithKeywordProperty("item_id"),
						WithTextAndKeywordProperty("title"),
						WithTextProperty("description"),
						WithTextProperty("content"),
						WithKeywordProperty("links"),
						WithDateNanosProperty("updatedParsed"),
						WithDateNanosProperty("publishedParsed"),
						WithObjectProperty("authors", map[string]types.Property{
							"name":  asTextAndKeyword(),
							"email": asTextAndKeyword(),
						}),
						WithKeywordProperty("guid"),
						WithObjectProperty("image", map[string]types.Property{
							"url":   types.NewKeywordProperty(),
							"title": asTextAndKeyword(),
						}),
						WithTextAndKeywordProperty("categories"),
						WithObjectProperty("enclosures", map[string]types.Property{
							"url":    types.NewKeywordProperty(),
							"length": types.NewKeywordProperty(),
							"type":   types.NewKeywordProperty(),
						}),
						WithObjectProperty("dublincoreext", DublinCoreMappingsTemplate()),
						WithObjectProperty("itunesext", ItunesMappingTemplate()),
						WithFlattenedProperty("extensions"),
						WithFlattenedProperty("custom"),
					),
				),
			),
		),
	)
}

// FeedItemsSettingsTemplate returns a Component Template for feed item
// index settings.
func FeedItemsSettingsTemplate() *putcomponenttemplate.Request {
	return NewComponentTemplateRequest(
		WithIndexOptions(
			NewIndexState(
				WithIndexSettings(
					WithIndexLifecycle(FeedItemsSchemaPrefix),
				),
			),
		),
	)
}

// IndexTemplateFeeds returns an Index Template for feed item indices.
func FeedItemsIndexTemplate() *putindextemplate.Request {
	return NewIndexTemplateRequest(
		WithIndexPatterns(FeedItemsSchemaPrefix+"_*"),
		WithComponentTemplates(FeedsItemsMappings, FeedsItemsSettings),
		WithPriority(500),
		AsDataStream(),
	)
}

// FeedItemsILMPolicy is the ILM policy for feed items indices.
func FeedItemsILMPolicy() *putlifecycle.Request {
	return DefaultILMPolicy()
}

// DefaultILMPolicy is a default ILM policy that will apply the following phases:
//
// - Hot Phase: rollover indices once they reach 50gb size.
//
// - Warm Phase: shrink and forcemerge indices down to 1 shard and 1 segment.
//
// - Delete Phase: delete indices older than 735 days.
func DefaultILMPolicy() *putlifecycle.Request {
	return NewILMPolicy(
		WithPhase(HotPhase,
			WithActions(WithRolloverMaxSize("50gb")),
		),
		WithPhase(WarmPhase,
			WithActions(
				WithShrinkToShards(1),
				WithForceMergeSegments(1),
			),
		),
		WithPhase(DeletePhase,
			WithMinAge("735d"),
			WithActions(WithDelete()),
		),
	)
}

func DublinCoreMappingsTemplate() map[string]types.Property {
	return map[string]types.Property{
		"title":       asTextAndKeyword(),
		"creator":     asTextAndKeyword(),
		"author":      asTextAndKeyword(),
		"subject":     asTextAndKeyword(),
		"description": asTextAndKeyword(),
		"publisher":   asTextAndKeyword(),
		"contributor": asTextAndKeyword(),
		"date":        asTextAndKeyword(),
		"type":        asTextAndKeyword(),
		"format":      asTextAndKeyword(),
		"identifier":  asTextAndKeyword(),
		"source":      asTextAndKeyword(),
		"language":    asTextAndKeyword(),
		"relation":    asTextAndKeyword(),
		"coverage":    asTextAndKeyword(),
		"rights":      asTextAndKeyword(),
	}
}

func ItunesMappingTemplate() map[string]types.Property {
	return map[string]types.Property{
		"author":            asTextAndKeyword(),
		"block":             asTextAndKeyword(),
		"duration":          asTextAndKeyword(),
		"explicit":          asTextAndKeyword(),
		"keywords":          asTextAndKeyword(),
		"subtitle":          asTextAndKeyword(),
		"summary":           asTextAndKeyword(),
		"image":             asTextAndKeyword(),
		"isClosedCaptioned": asTextAndKeyword(),
		"episode":           asTextAndKeyword(),
		"season":            asTextAndKeyword(),
		"order":             asTextAndKeyword(),
		"episodeType":       asTextAndKeyword(),
	}
}

// IngestPipelineFeeds is an ingest pipeline to clean-up feed and item data.
func IngestPipelineFeeds() *putpipeline.Request {
	return NewIngestPipeline(
		WithRemoveProcessor(
			RemoveDescription("Remove deprecated and/or unneeded fields"),
			RemoveFields("author", "published", "updated", "items"),
			RemoveIgnoreMissing(true),
		),
	)
}
