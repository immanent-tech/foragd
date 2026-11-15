/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package schema

import (
	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"

	"github.com/immanent-tech/foragd/providers/opensearch/indices"
)

var SubscriptionsMappings = indices.NewMappings(
	indices.WithMapping("subscription_id", indices.WithKeywordIgnoreAbove(32)),
	indices.WithMapping("user_id", indices.WithKeywordIgnoreAbove(32)),
	indices.WithMapping("type", indices.WithKeywordIgnoreAbove(24)),
	indices.WithMapping[opensearchapi.CommonMappingBooleanProperty]("favorite"),
	indices.WithMapping[opensearchapi.CommonMappingDateNanosProperty]("created_at"),
	indices.WithMapping[opensearchapi.CommonMappingDateNanosProperty]("updated_at"),
	indices.WithMapping[opensearchapi.CommonMappingDateNanosProperty]("marked_read_at"),
	indices.WithMapping("customisation",
		indices.WithObjectMappingDynamic(opensearchapi.NewCommonMappingDynamicFromBool(false)),
		indices.WithObjectMappingProperties(
			indices.WithMapping("nickname",
				indices.WithTextMappingProperties(
					indices.WithMapping("exact",
						indices.WithTextMappingAnalyzer(englishExactAnalyzer),
					),
					indices.WithMapping[opensearchapi.CommonMappingSearchAsYouTypeProperty]("search"),
				),
			),
			indices.WithMapping("categories",
				indices.WithTextMappingProperties(
					indices.WithMapping("exact",
						indices.WithTextMappingAnalyzer(englishExactAnalyzer),
					),
					indices.WithMapping[opensearchapi.CommonMappingSearchAsYouTypeProperty]("search"),
				),
			),
			indices.WithMapping("image_url", indices.WithKeywordIgnoreAbove(256)),
		),
	),
	indices.WithMapping[opensearchapi.CommonMappingFlatObjectProperty]("settings"),
	indices.WithMapping("feed_data",
		indices.WithObjectMappingDynamic(opensearchapi.NewCommonMappingDynamicFromBool(false)),
		indices.WithObjectMappingProperties(
			indices.WithMapping("feed_id", indices.WithKeywordIgnoreAbove(32)),
			indices.WithMapping[opensearchapi.CommonMappingFlatObjectProperty]("article_states"),
			ArticleFiltersMapping,
		),
	),
	indices.WithMapping("group_data",
		indices.WithObjectMappingDynamic(opensearchapi.NewCommonMappingDynamicFromBool(false)),
		indices.WithObjectMappingProperties(
			indices.WithMapping("subscriptions", indices.WithKeywordIgnoreAbove(32)),
			indices.WithMapping("metadata",
				indices.WithObjectMappingDynamic(opensearchapi.NewCommonMappingDynamicFromBool(false)),
				indices.WithObjectMappingProperties(
					indices.WithMapping("feed_id", indices.WithKeywordIgnoreAbove(32)),
					indices.WithMapping("subscription_id", indices.WithKeywordIgnoreAbove(32)),
				),
			),
			ArticleFiltersMapping,
		),
	),
	indices.WithMapping("email_data",
		indices.WithObjectMappingDynamic(opensearchapi.NewCommonMappingDynamicFromBool(false)),
		indices.WithObjectMappingProperties(
			indices.WithMapping("feed_id", indices.WithKeywordIgnoreAbove(32)),
			indices.WithMapping("email_sender_id", indices.WithKeywordIgnoreAbove(256)),
			indices.WithMapping[opensearchapi.CommonMappingFlatObjectProperty]("article_states"),
			ArticleFiltersMapping,
		),
	),
	indices.WithMapping[opensearchapi.CommonMappingFlatObjectProperty]("search_data"),
)

// SubscriptionsComponentTemplate is a component template containing [SubscriptionsMappings].
var SubscriptionsComponentTemplate = indices.NewComponentTemplateRequest(SubscriptionsIndex.String(),
	indices.WithIndexSettings(
		indices.WithMappings(SubscriptionsMappings),
		indices.WithAnalyzer(englishExactAnalyzer),
		indices.WithAnalyzer(htmlAnalyzer),
		indices.WithTokenizer(emailTokenizer),
	),
)

// SubscriptionsIndexTemplate is a index template for [models.Subscription].
var SubscriptionsIndexTemplate = indices.NewIndexTemplateRequest(SubscriptionsIndex.String(),
	indices.WithComponentTemplates(SubscriptionsIndex.String()),
	indices.WithIndexPatterns(SubscriptionsIndex.String()+"-*"),
	indices.WithIndexTemplatePriority(999),
)
