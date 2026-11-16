/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package schema

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/providers/opensearch"
	"github.com/immanent-tech/foragd/providers/opensearch/indices"
	"github.com/immanent-tech/foragd/providers/opensearch/ingest"
	"github.com/immanent-tech/foragd/providers/opensearch/ml"
)

const (
	// FeedsIndex is the index containing [model.Feed] documents.
	FeedsIndex opensearch.Index = "feeds"
	// ItemsIndex is the index containing [models.Item] documents.
	ItemsIndex opensearch.Index = "items"
	// FavoritesIndex is the index containing [models.Article] that a user has favorited.
	FavoritesIndex opensearch.Index = "favorites"
	// UsersIndex is the index containing [model.User] documents.
	UsersIndex opensearch.Index = "users"
	// SubscriptionsIndex is the index containing [models.Subscription] documents.
	SubscriptionsIndex opensearch.Index = "subscriptions"
	// SchedulerIndex is the index containing data for scheduled jobs and job data.
	SchedulerIndex opensearch.Index = "scheduler"
	// SessionsIndex is the index containing user session data.
	SessionsIndex opensearch.Index = "sessions"
	// ImportsIndex is the index containing user import jobs data.
	ImportsIndex opensearch.Index = "imports"
)

var AllIndices = []opensearch.Index{
	FeedsIndex,
	ItemsIndex,
	FavoritesIndex,
	UsersIndex,
	SubscriptionsIndex,
	SchedulerIndex,
	SessionsIndex,
	ImportsIndex,
}

type IndexConfig struct {
	index opensearch.Index
	// Name is the canonical name of the index.
	Name string
	// IndexTemplate is an index template for the index.
	IndexTemplate opensearchapi.IndicesPutIndexTemplateReq
	// ComponentTemplate is a component template for the index.
	ComponentTemplate opensearchapi.ClusterPutComponentTemplateReq
	// ReadAlias is the read-only index alias.
	ReadAlias string
	// WriteAlias is the read-write index alias.
	WriteAlias string
}

// NewIndexConfig generates an [IndexConfig] for the given [opensearch.Index]. This includes the appropriate
// canonical index name, index/component templates and read/write aliases for the index.
func NewIndexConfig(osc *opensearch.Client, index opensearch.Index) (IndexConfig, error) {
	var (
		indexName         string
		indexTemplate     opensearchapi.IndicesPutIndexTemplateReq
		componentTemplate opensearchapi.ClusterPutComponentTemplateReq
		readAlias         string
		writeAlias        string
	)
	switch index {
	case FeedsIndex:
		indexName = osc.IndexCanonicalName(index)
		indexTemplate = FeedsIndexTemplate
		componentTemplate = FeedsComponentTemplate
		readAlias = osc.IndexReadAlias(index)
		writeAlias = osc.IndexWriteAlias(index)
	case ItemsIndex:
		indexName = osc.IndexCanonicalName(index)
		indexTemplate = ItemsIndexTemplate
		componentTemplate = ItemsComponentTemplate
		readAlias = osc.IndexReadAlias(index)
		writeAlias = osc.IndexWriteAlias(index)
	case UsersIndex:
		indexName = osc.IndexCanonicalName(index)
		indexTemplate = UsersIndexTemplate
		componentTemplate = UsersComponentTemplate
		readAlias = osc.IndexReadAlias(index)
		writeAlias = osc.IndexWriteAlias(index)
	case FavoritesIndex:
		indexName = osc.IndexCanonicalName(index)
		indexTemplate = FavoritesIndexTemplate
		componentTemplate = FavoritesComponentTemplate
		readAlias = osc.IndexReadAlias(index)
		writeAlias = osc.IndexWriteAlias(index)
	case SubscriptionsIndex:
		indexName = osc.IndexCanonicalName(index)
		indexTemplate = SubscriptionsIndexTemplate
		componentTemplate = SubscriptionsComponentTemplate
		readAlias = osc.IndexReadAlias(index)
		writeAlias = osc.IndexWriteAlias(index)
	case SessionsIndex:
		indexName = osc.IndexCanonicalName(index)
		indexTemplate = SessionsIndexTemplate
		componentTemplate = SessionsComponentTemplate
		readAlias = osc.IndexReadAlias(index)
		writeAlias = osc.IndexWriteAlias(index)
	case SchedulerIndex:
		indexName = osc.IndexCanonicalName(index)
		indexTemplate = SchedulerIndexTemplate
		componentTemplate = SchedulerComponentTemplate
		readAlias = osc.IndexReadAlias(index)
		writeAlias = osc.IndexWriteAlias(index)
	case ImportsIndex:
		indexName = osc.IndexCanonicalName(index)
		indexTemplate = ImportsIndexTemplate
		componentTemplate = ImportsComponentTemplate
		readAlias = osc.IndexReadAlias(index)
		writeAlias = osc.IndexWriteAlias(index)
	default:
		return IndexConfig{}, fmt.Errorf("unknown index: %s", index)
	}
	return IndexConfig{
		Name:              indexName,
		IndexTemplate:     indexTemplate,
		ComponentTemplate: componentTemplate,
		ReadAlias:         readAlias,
		WriteAlias:        writeAlias,
	}, nil
}

// InitIndex will ensure all index templates and aliases exist and creates a new index ready for use.
func (c *IndexConfig) InitIndex(ctx context.Context, osc *opensearch.Client) error {
	// Create component template.
	if err := c.UpdateTemplates(ctx, osc); err != nil {
		return fmt.Errorf("update templates: %w", err)
	}

	// Create index.
	if err := indices.CreateIndexIfNotExists(ctx, osc.Index, c.Name); err != nil {
		return fmt.Errorf("create index %s: %w", c.Name, err)
	}
	slogctx.Info(ctx, "Created index",
		slog.String("name", c.Name),
	)

	// Create aliases.
	if err := c.UpdateAlias(ctx, osc, "write"); err != nil {
		return fmt.Errorf("update alias: %w", err)
	}
	if err := c.UpdateAlias(ctx, osc, "read"); err != nil {
		return fmt.Errorf("update alias: %w", err)
	}

	return nil
}

// UpdateIndex will update all templates for an index.
func (c *IndexConfig) UpdateIndex(ctx context.Context, osc *opensearch.Client) error {
	// Create component template.
	if err := c.UpdateTemplates(ctx, osc); err != nil {
		return fmt.Errorf("update templates: %w", err)
	}

	return nil
}

// UpdateTemplates will update the index and component templates for an index, and performs any index-specific
// configuration.
func (c *IndexConfig) UpdateTemplates(ctx context.Context, osc *opensearch.Client) error {
	// Perform any index specific actions.
	switch c.index {
	case ItemsIndex:
		mlc, err := ml.NewClient(osc)
		if err != nil {
			return fmt.Errorf("new ml client: %w", err)
		}
		if err := ingest.NewIngestPipeline(
			ctx,
			&osc.Ingest,
			ItemsIngestPipeline(mlc.GetModelID()),
		); err != nil {
			return fmt.Errorf("configure items ingest pipeline: %w", err)
		}
	}

	// Create/update component template.
	if err := indices.CreateComponentTemplate(ctx, osc.Cluster, c.ComponentTemplate); err != nil {
		return fmt.Errorf("create %s component template: %w", c.index, err)
	}
	slogctx.Info(ctx, "Created component template",
		slog.String("name", c.ComponentTemplate.Name),
	)

	// Create/update index template.
	if err := indices.CreateIndexTemplate(ctx, osc.Index, c.IndexTemplate); err != nil {
		return fmt.Errorf("create %s index template: %w", c.index, err)
	}
	slogctx.Info(ctx, "Created index template",
		slog.String("name", c.IndexTemplate.Name),
	)

	return nil
}

// UpdateAlias will update the read/write aliases for an index.
func (c *IndexConfig) UpdateAlias(ctx context.Context, osc *opensearch.Client, aliasType string) error {
	var (
		alias        string
		isWriteAlias bool
	)
	switch aliasType {
	case "read":
		alias = c.ReadAlias
	case "write":
		alias = c.WriteAlias
		isWriteAlias = true
	default:
		return fmt.Errorf("unknown alias type: %s", aliasType)
	}
	if err := indices.UpdateIndexAlias(ctx, osc.Index, alias, c.Name, isWriteAlias); err != nil {
		return fmt.Errorf("create %s alias %s->%s: %w", aliasType, alias, c.Name, err)
	}
	slogctx.Info(ctx, "Created alias",
		slog.String("type", aliasType),
		slog.String("alias", alias),
		slog.String("target", c.Name),
	)
	return nil
}
