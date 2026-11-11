/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package schema

import (
	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"

	"github.com/immanent-tech/foragd/providers/opensearch/indices"
)

// CommonMappings are field mappings common across [models.Item] and [models.Feed].
var CommonMappings = indices.NewMappings(
	indices.WithMapping[opensearchapi.CommonMappingDateNanosProperty]("published"),
	indices.WithMapping[opensearchapi.CommonMappingDateNanosProperty]("updated"),
	indices.WithMapping("title",
		indices.WithTextMappingProperties(
			indices.WithMapping[opensearchapi.CommonMappingKeywordProperty]("raw"),
			indices.WithMapping("exact",
				indices.WithTextMappingAnalyzer(englishExactAnalyzer),
			),
			indices.WithMapping[opensearchapi.CommonMappingSearchAsYouTypeProperty]("search"),
		),
	),
	indices.WithMapping("description", indices.WithTextMappingAnalyzer(htmlAnalyzer)),
	indices.WithMapping("authors",
		indices.WithTextMappingAnalyzer(personAnalyzer),
		indices.WithTextMappingProperties(
			indices.WithMapping[opensearchapi.CommonMappingKeywordProperty]("raw"),
			indices.WithMapping("exact",
				indices.WithTextMappingAnalyzer(englishExactAnalyzer),
			),
			indices.WithMapping[opensearchapi.CommonMappingSearchAsYouTypeProperty]("search"),
		),
	),
	indices.WithMapping("contributors",
		indices.WithTextMappingAnalyzer(personAnalyzer),
		indices.WithTextMappingProperties(
			indices.WithMapping[opensearchapi.CommonMappingKeywordProperty]("raw"),
			indices.WithMapping("exact",
				indices.WithTextMappingAnalyzer(englishExactAnalyzer),
			),
			indices.WithMapping[opensearchapi.CommonMappingSearchAsYouTypeProperty]("search"),
		),
	),
	indices.WithMapping("categories",
		indices.WithTextMappingProperties(
			indices.WithMapping[opensearchapi.CommonMappingKeywordProperty]("raw"),
			indices.WithMapping("raw_nocase",
				indices.WithKeywordMappingNormalizer(lowercaseNormalizer),
			),
			indices.WithMapping("exact",
				indices.WithTextMappingAnalyzer(englishExactAnalyzer),
			),
			indices.WithMapping[opensearchapi.CommonMappingSearchAsYouTypeProperty]("search"),
		),
	),
	indices.WithMapping[opensearchapi.CommonMappingKeywordProperty]("language"),
	indices.WithMapping[opensearchapi.CommonMappingTextProperty]("copyright"),
	indices.WithMapping[opensearchapi.CommonMappingKeywordProperty]("source_type"),
	indices.WithMapping[opensearchapi.CommonMappingKeywordProperty]("url"),
	indices.WithMapping("image",
		indices.WithObjectMappingDynamic(opensearchapi.NewCommonMappingDynamicFromBool(false)),
		indices.WithObjectMappingProperties(
			indices.WithMapping[opensearchapi.CommonMappingKeywordProperty]("url"),
			indices.WithMapping[opensearchapi.CommonMappingTextProperty]("title"),
		),
	),
)

func CommonComponentTemplate() opensearchapi.ClusterPutComponentTemplateReq {
	return indices.NewComponentTemplateRequest("common",
		indices.WithIndexSettings(
			indices.WithMappings(CommonMappings),
			indices.WithAnalyzer(englishExactAnalyzer),
			indices.WithAnalyzer(personAnalyzer),
			indices.WithAnalyzer(htmlAnalyzer),
			indices.WithTokenizer(emailTokenizer),
			indices.WithNormalizer(lowercaseNormalizer),
		),
	)
}
