/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package schema

import (
	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"

	"github.com/immanent-tech/foragd/providers/opensearch/indices"
)

// SessionsMappings is the field mappings for [models.Session].
var SessionsMappings = indices.NewMappings(
	indices.WithMapping[opensearchapi.CommonMappingDateNanosProperty]("created_at"),
	indices.WithMapping[opensearchapi.CommonMappingDateNanosProperty]("expiry"),
	indices.WithMapping[opensearchapi.CommonMappingKeywordProperty]("token"),
	indices.WithMapping[opensearchapi.CommonMappingBinaryProperty]("data"),
)

// SessionsComponentTemplate is a component template containing [SchedulerMappings].
var SessionsComponentTemplate = indices.NewComponentTemplateRequest(SessionsIndex.String(),
	indices.WithIndexSettings(
		indices.WithMappings(SessionsMappings),
	),
)

// SessionsIndexTemplate is a index template for [models.Feed].
var SessionsIndexTemplate = indices.NewIndexTemplateRequest(SessionsIndex.String(),
	indices.WithComponentTemplates(SessionsIndex.String()),
	indices.WithIndexPatterns(SessionsIndex.String()+"-*"),
	indices.WithIndexTemplatePriority(999),
)
