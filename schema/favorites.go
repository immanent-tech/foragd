/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package schema

import (
	"github.com/immanent-tech/foragd/providers/opensearch/indices"
)

var FavoritesMappings = indices.NewMappings(
	indices.WithMapping("subscription_id", indices.WithKeywordIgnoreAbove(32)),
	indices.WithMapping("user_id", indices.WithKeywordIgnoreAbove(32)),
)

// FavoritesComponentTemplate is a component template containing [FavoritesMappings].
var FavoritesComponentTemplate = indices.NewComponentTemplateRequest(FavoritesIndex.String(),
	indices.WithIndexSettings(
		indices.WithMappings(FavoritesMappings),
	),
)

// FavoritesIndexTemplate is a index template for user favorites of [models.Item].
var FavoritesIndexTemplate = indices.NewIndexTemplateRequest(FavoritesIndex.String(),
	indices.WithComponentTemplates("common", ItemsIndex.String(), FavoritesIndex.String()),
	indices.WithIndexPatterns(FavoritesIndex.String()+"-*"),
	indices.WithIndexTemplatePriority(999),
)
