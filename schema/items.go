/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package schema

import (
	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"

	"github.com/immanent-tech/foragd/providers/opensearch/indices"
)

var ItemsMappings = indices.NewMappings(
	indices.WithMapping[opensearchapi.CommonMappingDateNanosProperty]("@timestamp"),
	indices.WithMapping[opensearchapi.CommonMappingDateNanosProperty]("created"),
	indices.WithMapping[opensearchapi.CommonMappingKeywordProperty]("feed_id"),
	indices.WithMapping[opensearchapi.CommonMappingKeywordProperty]("item_id"),
	indices.WithMapping("feed_title",
		indices.WithTextMappingProperties(
			indices.WithMapping[opensearchapi.CommonMappingKeywordProperty]("raw"),
			indices.WithMapping("exact",
				indices.WithTextMappingAnalyzer(englishExactAnalyzer),
			),
			indices.WithMapping[opensearchapi.CommonMappingSearchAsYouTypeProperty]("search"),
		),
	),
	indices.WithMapping("content",
		indices.WithTextMappingAnalyzer(htmlAnalyzer),
	),
	indices.WithMapping[opensearchapi.CommonMappingKeywordProperty]("extension_type"),
	indices.WithMapping[opensearchapi.CommonMappingFlatObjectProperty]("extension_data"),
	indices.WithMapping("geo",
		indices.WithObjectMappingDynamic(opensearchapi.NewCommonMappingDynamicFromBool(false)),
		indices.WithObjectMappingProperties(
			indices.WithMapping[opensearchapi.CommonMappingGeoPointProperty]("point"),
		),
	),
	indices.WithMapping[opensearchapi.CommonMappingKeywordProperty]("comments_url"),
	indices.WithMapping[opensearchapi.CommonMappingKeywordProperty]("comments_rss"),
)

// ItemsComponentTemplate is a component template containing [ItemMappings].
var ItemsComponentTemplate = indices.NewComponentTemplateRequest(ItemsIndex.String(),
	indices.WithIndexSettings(
		indices.WithMappings(ItemsMappings),
		indices.WithAnalyzer(englishExactAnalyzer),
		indices.WithAnalyzer(htmlAnalyzer),
		indices.WithTokenizer(emailTokenizer),
		indices.WithReplicas("2"),
		indices.WithKNN(),
		indices.WithDefaultPipeline(ItemsIndex.String()+"_pipeline"),
	),
)

// ItemsIndexTemplate is a index template for [models.Item].
var ItemsIndexTemplate = indices.NewIndexTemplateRequest(ItemsIndex.String(),
	indices.WithComponentTemplates("common", ItemsIndex.String()),
	indices.WithIndexPatterns(ItemsIndex.String()+"-*"),
	indices.WithIndexTemplatePriority(999),
)

// ItemsIngestPipeline defines an ingest pipeline for processing [models.Item] before indexing.
func ItemsIngestPipeline(modelID string) opensearchapi.IngestPutPipelineReq {
	textEmbeddingProcessor := opensearchapi.IngestProcessorContainer{
		TextEmbedding: &opensearchapi.IngestTextEmbeddingProcessor{
			ModelID: modelID,
			FieldMap: map[string]string{
				"content": "content_semantic",
			},
		},
	}

	return opensearchapi.IngestPutPipelineReq{
		ID: ItemsIndex.String() + "-pipeline",
		Body: &opensearchapi.IngestPutPipelineBody{
			Description: new("Ingest pipeline for feed items"),
			Processors:  []opensearchapi.IngestProcessorContainer{textEmbeddingProcessor},
		},
	}
}

var ItemsISM = opensearchapi.ISMPolicy{
	ISMTemplate: new(opensearchapi.NewISMPolicyTemplateFromISMTemplate(
		opensearchapi.ISMTemplate{
			IndexPatterns: []string{ItemsIndex.String() + "-*"},
			Priority:      new(100.0),
		},
	)),
	States: []opensearchapi.ISMStates{
		{
			Name: new("hot"),
			Actions: []opensearchapi.ISMAction{
				{Rollover: &opensearchapi.ISMActionRollover{MinSize: new("10gb")}},
			},
			Transitions: []opensearchapi.ISMTransition{
				{StateName: new("merge")},
			},
		},
		{
			Name: new("merge"),
			Actions: []opensearchapi.ISMAction{
				{ForceMerge: &opensearchapi.ISMActionForceMerge{MaxNumSegments: new(1)}},
			},
			// Transitions: []opensearchapi.ISMTransition{
			// 	{StateName: new("delete"), Conditions: map[string]jsonx.RawMessage{}},
			// },
		},
		// {
		// 	Name: new("delete"),
		// 	Actions: []opensearchapi.ISMAction{
		// 		{Delete: &opensearchapi.ISMActionDelete{}},
		// 	},
		// },
	},
}
