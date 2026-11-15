/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package schema

import (
	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"

	"github.com/immanent-tech/foragd/providers/opensearch/indices"
)

var FeedsMappings = indices.NewMappings(
	indices.WithMapping("feed_id", indices.WithKeywordIgnoreAbove(32)),
	indices.WithMapping[opensearchapi.CommonMappingDateNanosProperty]("created_at"),
	indices.WithMapping[opensearchapi.CommonMappingDateNanosProperty]("last_fetched"),
	indices.WithMapping[opensearchapi.CommonMappingUnsignedLongNumberProperty]("update_interval"),
	indices.WithMapping[opensearchapi.CommonMappingBooleanProperty]("updates_disabled"),
	indices.WithMapping("source_urls", indices.WithKeywordIgnoreAbove(256)),
	indices.WithMapping("fetch_method", indices.WithKeywordIgnoreAbove(24)),
	indices.WithMapping[opensearchapi.CommonMappingFlatObjectProperty]("fetch_options"),
	indices.WithMapping[opensearchapi.CommonMappingFlatObjectProperty]("quirks"),
	indices.WithMapping("domain",
		indices.WithTextMappingAnalyzer(domainNameAnalyzer),
		indices.WithTextMappingProperties(
			indices.WithMapping("raw", indices.WithKeywordIgnoreAbove(256)),
			indices.WithMapping[opensearchapi.CommonMappingSearchAsYouTypeProperty]("search"),
		),
	),
	indices.WithMapping[opensearchapi.CommonMappingFlatObjectProperty]("source_data"),
	indices.WithMapping("customisation",
		indices.WithObjectMappingDynamic(opensearchapi.NewCommonMappingDynamicFromBool(false)),
		indices.WithObjectMappingProperties(
			indices.WithMapping("title",
				indices.WithTextMappingProperties(
					indices.WithMapping("exact",
						indices.WithTextMappingAnalyzer(englishExactAnalyzer),
					),
					indices.WithMapping[opensearchapi.CommonMappingSearchAsYouTypeProperty]("search"),
				),
			),
			indices.WithMapping("description", indices.WithTextMappingAnalyzer(htmlAnalyzer)),
		),
	),
)

// FeedsComponentTemplate is a component template containing [FeedMappings].
var FeedsComponentTemplate = indices.NewComponentTemplateRequest(FeedsIndex.String(),
	indices.WithIndexSettings(
		indices.WithMappings(FeedsMappings),
		indices.WithAnalyzer(englishExactAnalyzer),
		indices.WithAnalyzer(domainNameAnalyzer),
		indices.WithTokenizer(domainTokenizer),
		indices.WithAnalyzer(htmlAnalyzer),
		indices.WithTokenizer(emailTokenizer),
	),
)

// FeedsIndexTemplate is a index template for [models.Feed].
var FeedsIndexTemplate = indices.NewIndexTemplateRequest(FeedsIndex.String(),
	indices.WithComponentTemplates("common", FeedsIndex.String()),
	indices.WithIndexPatterns(FeedsIndex.String()+"-*"),
	indices.WithIndexTemplatePriority(999),
)
