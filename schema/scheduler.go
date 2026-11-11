/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package schema

import (
	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"

	"github.com/immanent-tech/foragd/providers/opensearch/indices"
)

// SchedulerMappings is the field mappings for [models.Session].
var SchedulerMappings = indices.NewMappings(
	indices.WithMapping[opensearchapi.CommonMappingDateNanosProperty]("created_at"),
	indices.WithMapping[opensearchapi.CommonMappingDateNanosProperty]("updated_at"),
	indices.WithMapping[opensearchapi.CommonMappingFlatObjectProperty]("job_options"),
	indices.WithMapping[opensearchapi.CommonMappingFlatObjectProperty]("job_data"),
	indices.WithMapping[opensearchapi.CommonMappingKeywordProperty]("job_type"),
	indices.WithMapping[opensearchapi.CommonMappingKeywordProperty]("job_key"),
	indices.WithMapping[opensearchapi.CommonMappingTextProperty]("job_description"),
	indices.WithMapping[opensearchapi.CommonMappingKeywordProperty]("job_trigger_type"),
	indices.WithMapping[opensearchapi.CommonMappingFlatObjectProperty]("job_trigger"),
	indices.WithMapping[opensearchapi.CommonMappingDateNanosProperty]("job_next_run"),
)

// SchedulerComponentTemplate is a component template containing [SchedulerMappings].
var SchedulerComponentTemplate = indices.NewComponentTemplateRequest(SchedulerIndex.String(),
	indices.WithIndexSettings(
		indices.WithMappings(SchedulerMappings),
	),
)

// SchedulerIndexTemplate is a index template for [models.Feed].
var SchedulerIndexTemplate = indices.NewIndexTemplateRequest(SchedulerIndex.String(),
	indices.WithComponentTemplates(SchedulerIndex.String()),
	indices.WithIndexPatterns(SchedulerIndex.String()+"-*"),
	indices.WithIndexTemplatePriority(999),
)
