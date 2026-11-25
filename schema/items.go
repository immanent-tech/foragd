/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package schema

import (
	"bytes"
	"encoding/json"

	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"

	"github.com/immanent-tech/foragd/providers/opensearch/indices"
)

// ItemsMappings defines the field mappings for [models.Item] indices.
var ItemsMappings = func(modelID string) indices.Mappings {
	return indices.NewMappings(
		indices.WithMapping[opensearchapi.CommonMappingDateNanosProperty]("@timestamp"),
		indices.WithMapping[opensearchapi.CommonMappingDateNanosProperty]("created"),
		indices.WithMapping("feed_id", indices.WithKeywordIgnoreAbove(32)),
		indices.WithMapping("item_id", indices.WithKeywordIgnoreAbove(32)),
		indices.WithMapping("feed_title",
			indices.WithTextMappingProperties(
				indices.WithMapping("exact",
					indices.WithTextMappingAnalyzer(englishExactAnalyzer),
				),
				indices.WithMapping[opensearchapi.CommonMappingSearchAsYouTypeProperty]("search"),
			),
		),
		indices.WithMapping("content",
			indices.WithTextMappingAnalyzer(htmlAnalyzer),
		),
		indices.WithMapping("content_semantic",
			indices.WithSemanticMappingModelID(modelID)),
		indices.WithMapping("extension_type", indices.WithKeywordIgnoreAbove(256)),
		indices.WithMapping[opensearchapi.CommonMappingFlatObjectProperty]("extension_data"),
		indices.WithMapping("geo",
			indices.WithObjectMappingDynamic(opensearchapi.NewCommonMappingDynamicFromBool(false)),
			indices.WithObjectMappingProperties(
				indices.WithMapping[opensearchapi.CommonMappingGeoPointProperty]("point"),
			),
		),
		indices.WithMapping("comments_url", indices.WithKeywordIgnoreAbove(256)),
		indices.WithMapping("comments_rss", indices.WithKeywordIgnoreAbove(256)),
	)
}

// ItemsComponentTemplate is a component template containing [ItemMappings].
var ItemsComponentTemplate = func(modelID, readAlias string) opensearchapi.ClusterPutComponentTemplateReq {
	return indices.NewComponentTemplateRequest(ItemsIndex.String(),
		indices.WithIndexSettings(
			indices.WithMappings(ItemsMappings(modelID)),
			indices.WithAnalyzer(englishExactAnalyzer),
			indices.WithAnalyzer(htmlAnalyzer),
			indices.WithTokenizer(emailTokenizer),
			indices.WithReplicas("2"),
			indices.WithKNN(),
			indices.WithDefaultPipeline(ItemsIndex.String()+"-pipeline"),
			indices.WithAlias(readAlias, nil),
		),
	)
}

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

// ItemsISMIndexTemplate is an index template to define the rollover alias required for the associated items [ItemsISM]
// ISM policy.
var ItemsISMIndexTemplate = func(writeAlias string) opensearchapi.IndicesPutIndexTemplateReq {
	type body struct {
		IndexPatterns []string `json:"index_patterns"`
		Template      struct {
			Settings struct {
				Plugins struct {
					IndexStateManagement struct {
						RolloverAlias string `json:"rollover_alias"`
					} `json:"index_state_management"`
				} `json:"plugins"`
			} `json:"settings"`
		} `json:"template"`
		Prority int `json:"priority"`
	}

	var settings body
	settings.IndexPatterns = []string{ItemsIndex.String() + "-*"}
	settings.Template.Settings.Plugins.IndexStateManagement.RolloverAlias = writeAlias
	settings.Prority = 1000

	data, _ := json.Marshal(&settings)
	return opensearchapi.IndicesPutIndexTemplateReq{
		Name:       ItemsIndex.String() + "-ism",
		BodyReader: bytes.NewReader(data),
	}
}

// ItemsISM is an Index State Management policy to maintain reasonable sizing and history of items indices.
var ItemsISM = opensearchapi.ISMPolicy{
	PolicyID:     new(ItemsIndex.String() + "-ism-policy"),
	Description:  new("ISM Policy to maintain reasonable sizing and history of items indices"),
	DefaultState: new("hot"),
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
			Transitions: []opensearchapi.ISMTransition{
				{
					StateName:  new("delete"),
					Conditions: map[string]json.RawMessage{"min_index_age": json.RawMessage(`"365d"`)},
				},
			},
		},
		{
			Name: new("delete"),
			Actions: []opensearchapi.ISMAction{
				{Delete: &opensearchapi.ISMActionDelete{}},
			},
		},
	},
}
