/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package schema

import (
	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"

	"github.com/immanent-tech/foragd/providers/opensearch/indices"
)

// UsersMappings is the field mappings for [models.User].
var UsersMappings = indices.NewMappings(
	indices.WithMapping("user_id", indices.WithKeywordIgnoreAbove(32)),
	indices.WithMapping("nickname", indices.WithKeywordIgnoreAbove(128)),
	indices.WithMapping("avatar_url", indices.WithKeywordIgnoreAbove(256)),
	indices.WithMapping("external_user_id", indices.WithKeywordIgnoreAbove(64)),
	indices.WithMapping("email", indices.WithKeywordIgnoreAbove(128)),
	indices.WithMapping("provider", indices.WithKeywordIgnoreAbove(32)),
	indices.WithMapping("level", indices.WithKeywordIgnoreAbove(32)),
	indices.WithMapping[opensearchapi.CommonMappingDateNanosProperty]("created_at"),
	indices.WithMapping[opensearchapi.CommonMappingDateNanosProperty]("updated_at"),
	indices.WithMapping[opensearchapi.CommonMappingDateNanosProperty]("last_login"),
	indices.WithMapping[opensearchapi.CommonMappingUnsignedLongNumberProperty]("login_count"),
	indices.WithMapping("max_history", indices.WithKeywordIgnoreAbove(32)),
	indices.WithMapping[opensearchapi.CommonMappingFlatObjectProperty]("settings"),
	indices.WithMapping("subscription_type", indices.WithKeywordIgnoreAbove(32)),
	indices.WithMapping[opensearchapi.CommonMappingFlatObjectProperty]("subscription"),
	indices.WithMapping("customisation",
		indices.WithObjectMappingDynamic(opensearchapi.NewCommonMappingDynamicFromBool(false)),
		indices.WithObjectMappingProperties(
			indices.WithMapping[opensearchapi.CommonMappingBooleanProperty]("blocked"),
			indices.WithMapping[opensearchapi.CommonMappingBooleanProperty]("email_verified"),
			indices.WithMapping[opensearchapi.CommonMappingBooleanProperty]("policies_accepted"),
			indices.WithMapping[opensearchapi.CommonMappingBooleanProperty]("pending_deletion"),
			indices.WithMapping("susbcription_limit",
				indices.WithObjectMappingDynamic(opensearchapi.NewCommonMappingDynamicFromBool(false)),
				indices.WithObjectMappingProperties(
					indices.WithMapping[opensearchapi.CommonMappingBooleanProperty]("exceeded"),
					indices.WithMapping[opensearchapi.CommonMappingDateNanosProperty]("timestamp"),
				),
			),
			indices.WithMapping("newsletter_limit",
				indices.WithObjectMappingDynamic(opensearchapi.NewCommonMappingDynamicFromBool(false)),
				indices.WithObjectMappingProperties(
					indices.WithMapping[opensearchapi.CommonMappingBooleanProperty]("exceeded"),
					indices.WithMapping[opensearchapi.CommonMappingDateNanosProperty]("timestamp"),
				),
			),
		),
	),
	indices.WithMapping("item_favorites", indices.WithKeywordIgnoreAbove(32)),
)

// UsersComponentTemplate is a component template containing [UsersMappings].
var UsersComponentTemplate = indices.NewComponentTemplateRequest(UsersIndex.String(),
	indices.WithIndexSettings(
		indices.WithMappings(UsersMappings),
		indices.WithReplicas("2"),
	),
)

// UsersIndexTemplate is a index template for [models.User].
var UsersIndexTemplate = indices.NewIndexTemplateRequest(UsersIndex.String(),
	indices.WithComponentTemplates(UsersIndex.String()),
	indices.WithIndexPatterns(UsersIndex.String()+"-*"),
	indices.WithIndexTemplatePriority(999),
)
