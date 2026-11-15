/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package schema

import (
	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"

	"github.com/immanent-tech/foragd/providers/opensearch/indices"
)

var ImportsMappings = indices.NewMappings(
	indices.WithMapping[opensearchapi.CommonMappingDateNanosProperty]("created_at"),
	indices.WithMapping[opensearchapi.CommonMappingDateNanosProperty]("updated_at"),
	indices.WithMapping("doc_type", indices.WithKeywordIgnoreAbove(32)),
	indices.WithMapping("job_id", indices.WithKeywordIgnoreAbove(32)),
	indices.WithMapping("user_id", indices.WithKeywordIgnoreAbove(32)),
	indices.WithMapping("status", indices.WithKeywordIgnoreAbove(24)),
	indices.WithMapping("error",
		indices.WithObjectMappingDynamic(opensearchapi.NewCommonMappingDynamicFromBool(false)),
		indices.WithObjectMappingProperties(
			indices.WithMapping[opensearchapi.CommonMappingTextProperty]("summary"),
			indices.WithMapping[opensearchapi.CommonMappingTextProperty]("details"),
			indices.WithMapping("status", indices.WithKeywordIgnoreAbove(24)),
		),
	),
	indices.WithMapping("requests",
		indices.WithObjectMappingDynamic(opensearchapi.NewCommonMappingDynamicFromBool(false)),
		indices.WithObjectMappingProperties(
			indices.WithMapping("url", indices.WithKeywordIgnoreAbove(256)),
			indices.WithMapping("categories", indices.WithKeywordIgnoreAbove(256)),
		),
	),
	indices.WithMapping("url", indices.WithKeywordIgnoreAbove(256)),
	indices.WithMapping("feed_id", indices.WithKeywordIgnoreAbove(32)),
	indices.WithMapping("subscription_id", indices.WithKeywordIgnoreAbove(32)),
	indices.WithMapping("categories", indices.WithKeywordIgnoreAbove(256)),
)

// ImportsComponentTemplate is a component template containing [ImportsMappings].
var ImportsComponentTemplate = indices.NewComponentTemplateRequest(ImportsIndex.String(),
	indices.WithIndexSettings(
		indices.WithMappings(ImportsMappings),
	),
)

// ImportsIndexTemplate is a index template for [models.ImportStatus].
var ImportsIndexTemplate = indices.NewIndexTemplateRequest(ImportsIndex.String(),
	indices.WithComponentTemplates(ImportsIndex.String()),
	indices.WithIndexPatterns(ImportsIndex.String()+"-*"),
	indices.WithIndexTemplatePriority(999),
)
