// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"

	"github.com/joshuar/go-feed-me/internal/platforms/elastic/schema"
)

var ErrMigrationFailed = errors.New("schema migration failed")

// Migration will create all necessary index templates settings and policies.
func (c *Client) Migration(ctx context.Context) error {
	c.Logger.Debug("Migrating feeds index field mappings...")

	if _, err := c.API.Cluster.PutComponentTemplate(schema.FeedsMappings).
		Request(schema.ComponentTemplateFeedsMappings()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	c.Logger.Debug("Migrating feeds index settings...")

	if _, err := c.API.Cluster.PutComponentTemplate(schema.FeedsSettings).
		Request(schema.ComponentTemplateFeedsSettings()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	c.Logger.Debug("Migrating feeds index template...")

	if _, err := c.API.Indices.PutIndexTemplate(schema.FeedSchemaPrefix).
		Request(schema.IndexTemplateFeeds()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	c.Logger.Debug("Migrating feed items ILM policy...")

	if _, err := c.API.Ilm.PutLifecycle(schema.FeedItemsSchemaPrefix).
		Request(schema.ILMPolicyFeedItems()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	c.Logger.Debug("Migrating feed items index field mappings...")

	if _, err := c.API.Cluster.PutComponentTemplate(schema.FeedsItemsMappings).
		Request(schema.ComponentTemplateFeedItemsMappings()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	c.Logger.Debug("Migrating feed items index field settings...")

	if _, err := c.API.Cluster.PutComponentTemplate(schema.FeedsItemsSettings).
		Request(schema.ComponentTemplateFeedItemsSettings()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	c.Logger.Debug("Migrating feed items index template...")

	if _, err := c.API.Indices.PutIndexTemplate(schema.FeedItemsSchemaPrefix).
		Request(schema.IndexTemplateFeedItems()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	c.Logger.Debug("Migrating read items ILM policy...")

	if _, err := c.API.Ilm.PutLifecycle(schema.ReadItemsSchemaPrefix).
		Request(schema.ILMPolicyReadItems()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	c.Logger.Debug("Migrating read items index field mappings...")

	if _, err := c.API.Cluster.PutComponentTemplate(schema.ReadItemsMappings).
		Request(schema.ComponentTemplateReadItemsMappings()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	c.Logger.Debug("Migrating read items index field settings...")

	if _, err := c.API.Cluster.PutComponentTemplate(schema.ReadItemsSettings).
		Request(schema.ComponentTemplateReadItemsSettings()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	c.Logger.Debug("Migrating read items index template...")

	if _, err := c.API.Indices.PutIndexTemplate(schema.ReadItemsSchemaPrefix).
		Request(schema.IndexTemplateReadItems()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	c.Logger.Debug("Migrating ingest pipeline...")

	if _, err := c.API.Ingest.PutPipeline(schema.IngestPipelineID).
		Request(schema.IngestPipelineFeeds()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	return nil
}
