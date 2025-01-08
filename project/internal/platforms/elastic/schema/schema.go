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
	MappingsSuffix            = "_mappings"
	SettingsSuffix            = "_settings"
	SubscriptionsSchemaPrefix = "subscriptions"
	FeedsSchemaPrefix         = "feeds"
	FeedItemsSchemaPrefix     = "feeditems"
	ReadItemsSchemaPrefix     = "readitems"

	IngestPipelineID = "gofeed"

	SubscriptionsMappings = SubscriptionsSchemaPrefix + MappingsSuffix
	SubscriptionsSettings = SubscriptionsSchemaPrefix + SettingsSuffix
	FeedsMappings         = FeedsSchemaPrefix + MappingsSuffix
	FeedsSettings         = FeedsSchemaPrefix + SettingsSuffix
	FeedsItemsMappings    = FeedItemsSchemaPrefix + MappingsSuffix
	FeedsItemsSettings    = FeedItemsSchemaPrefix + SettingsSuffix
	ReadItemsMappings     = ReadItemsSchemaPrefix + MappingsSuffix
	ReadItemsSettings     = ReadItemsSchemaPrefix + SettingsSuffix

	schemaVersion = "v0.0.0"
)

var defaultMetadata = NewMetadata(WithMetadataField("version", schemaVersion))

// Option is a reusable generic function for applying options to a type.
type Option[T any] func(T) T

//
// SUBSCRIPTIONS
//

// ComponentTemplateSubscriptionsMappings returns a Component Template for subscriptions
// index field mappings.
func ComponentTemplateSubscriptionsMappings() *putcomponenttemplate.Request {
	return NewComponentTemplateRequest(
		WithIndexOptions(
			NewIndexState(
				WithMappings(
					NewPropertyMapping(
						WithDateNanosProperty("@timestamp"),
						WithKeywordProperty("feed_id"),
						WithKeywordProperty("subscription_id"),
						WithKeywordProperty("user_id"),
						WithTextAndKeywordProperty("name"),
						WithTextAndKeywordProperty("categories"),
					),
				),
			),
		),
	)
}

// ComponentTemplateSubscriptionsSettings returns a Component Template for subscriptions
// index settings.
func ComponentTemplateSubscriptionsSettings() *putcomponenttemplate.Request {
	return NewComponentTemplateRequest(
		WithIndexOptions(
			NewIndexState(
				WithAliases("subscriptions", types.Alias{}),
			),
		),
	)
}

// IndexTemplateFeeds returns an Index Template for feeds indices.
func IndexTemplateSubscriptions() *putindextemplate.Request {
	return NewIndexTemplateRequest(
		WithIndexPatterns(SubscriptionsSchemaPrefix+"-*"),
		WithComponentTemplates(SubscriptionsMappings, SubscriptionsSettings),
	)
}

//
// FEEDS
//

// ComponentTemplateFeedsMappings returns a Component Template for feeds
// index field mappings.
func ComponentTemplateFeedsMappings() *putcomponenttemplate.Request {
	return NewComponentTemplateRequest(
		WithIndexOptions(
			NewIndexState(
				WithMappings(
					NewPropertyMapping(
						WithDateNanosProperty("@timestamp"),
						WithKeywordProperty("feed_id"),
						WithDateNanosProperty("created_at"),
						WithTextAndKeywordProperty("title"),
						WithTextProperty("description"),
						WithTextProperty("content"),
						WithKeywordProperty("link"),
						WithKeywordProperty("feedlink"),
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
						WithObjectProperty("dublincoreext", DublinCoreMapping()),
						WithObjectProperty("itunesext", ItunesMapping()),
						WithFlattenedProperty("extensions"),
						WithFlattenedProperty("custom"),
					),
				),
			),
		),
	)
}

// ComponentTemplateFeedsSettings returns a Component Template for feeds
// index settings.
func ComponentTemplateFeedsSettings() *putcomponenttemplate.Request {
	return NewComponentTemplateRequest(
		WithIndexOptions(
			NewIndexState(
				WithAliases("feeds", types.Alias{}),
			),
		),
	)
}

// IndexTemplateFeeds returns an Index Template for feeds indices.
func IndexTemplateFeeds() *putindextemplate.Request {
	return NewIndexTemplateRequest(
		WithIndexPatterns(FeedsSchemaPrefix+"-*"),
		WithComponentTemplates(FeedsMappings, FeedsSettings),
	)
}

//
// FEED ITEMS
//

// ComponentTemplateFeedsMappings returns a Component Template for feed items
// index field mappings.
func ComponentTemplateFeedItemsMappings() *putcomponenttemplate.Request {
	return NewComponentTemplateRequest(
		WithIndexOptions(
			NewIndexState(
				WithMappings(
					NewPropertyMapping(
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
						WithObjectProperty("dublincoreext", DublinCoreMapping()),
						WithObjectProperty("itunesext", ItunesMapping()),
						WithFlattenedProperty("extensions"),
						WithFlattenedProperty("custom"),
					),
				),
			),
		),
	)
}

// ComponentTemplateFeedItemsSettings returns a Component Template for feed item
// index settings.
func ComponentTemplateFeedItemsSettings() *putcomponenttemplate.Request {
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
func IndexTemplateFeedItems() *putindextemplate.Request {
	return NewIndexTemplateRequest(
		WithIndexPatterns(FeedItemsSchemaPrefix+"-*"),
		WithComponentTemplates(FeedsItemsMappings, FeedsItemsSettings),
		AsDataStream(),
	)
}

// ILMPolicyFeedItems is the ILM policy for feed items indices.
func ILMPolicyFeedItems() *putlifecycle.Request {
	return DefaultILMPolicy()
}

//
// READ ITEMS
//

// ComponentTemplateReadItemsMappings returns a Component Template for read items
// index field mappings.
func ComponentTemplateReadItemsMappings() *putcomponenttemplate.Request {
	return NewComponentTemplateRequest(
		WithIndexOptions(
			NewIndexState(
				WithMappings(
					NewPropertyMapping(
						WithDateNanosProperty("@timestamp"),
						WithKeywordProperty("feed_id"),
						WithKeywordProperty("item_id"),
						WithKeywordProperty("user_id"),
					),
				),
			),
		),
	)
}

// ComponentTemplateReadItemsSettings returns a Component Template for read item
// index settings.
func ComponentTemplateReadItemsSettings() *putcomponenttemplate.Request {
	return NewComponentTemplateRequest(
		WithIndexOptions(
			NewIndexState(
				WithIndexSettings(
					WithIndexLifecycle(ReadItemsSchemaPrefix),
				),
			),
		),
	)
}

// IndexTemplateReadItems returns an Index Template for read items indices.
func IndexTemplateReadItems() *putindextemplate.Request {
	return NewIndexTemplateRequest(
		WithIndexPatterns(ReadItemsSchemaPrefix+"-*"),
		WithComponentTemplates(ReadItemsMappings, ReadItemsSettings),
		AsDataStream(),
	)
}

// ILMPolicyReadItems is the ILM policy for read items indices.
func ILMPolicyReadItems() *putlifecycle.Request {
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

func DublinCoreMapping() map[string]types.Property {
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

func ItunesMapping() map[string]types.Property {
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
