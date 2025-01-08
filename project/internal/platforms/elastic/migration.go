// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//nolint:dupl
package elastic

import (
	"context"
	"errors"

	"github.com/joshuar/go-feed-me/internal/platforms/elastic/schema"
)

var validMigrations = []string{"feeds", "subscriptions", "feeditems", "readitems", "ingest"}

var ErrMigrationFailed = errors.New("schema migration failed")

// Migration will create all necessary index templates settings and policies.
func (c *Client) Migration(ctx context.Context, migrations ...string) error {
	// If no migrations are specified, perform migrations for all items.
	if len(migrations) == 0 {
		migrations = validMigrations
	}

	// Perform requested migrations.
	for _, migration := range migrations {
		var err error

		switch migration {
		case "subscriptions":
			err = c.migrateSubscriptions(ctx)
		case "feeds":
			err = c.migrateFeeds(ctx)
		case "feeditems":
			err = c.migrateFeedItems(ctx)
		case "readitems":
			err = c.migratedReadItems(ctx)
		case "ingest":
			err = c.migrateIngest(ctx)
		}

		if err != nil {
			return errors.Join(ErrMigrationFailed, err)
		}
	}

	return nil
}

// migrateSubscriptions contains migration actions for migrating subscriptions indices and
// settings.
func (c *Client) migrateSubscriptions(ctx context.Context) error {
	c.Logger.Debug("Migrating subscriptions...")

	if _, err := c.API.Cluster.PutComponentTemplate(schema.SubscriptionsMappings).
		Request(schema.ComponentTemplateSubscriptionsMappings()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if _, err := c.API.Cluster.PutComponentTemplate(schema.SubscriptionsSettings).
		Request(schema.ComponentTemplateSubscriptionsSettings()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if _, err := c.API.Indices.PutIndexTemplate(schema.SubscriptionsSchemaPrefix).
		Request(schema.IndexTemplateSubscriptions()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	return nil
}

// migrateFeeds contains migration actions for migrating feeds indices and
// settings.
func (c *Client) migrateFeeds(ctx context.Context) error {
	c.Logger.Debug("Migrating feeds...")

	if _, err := c.API.Cluster.PutComponentTemplate(schema.FeedsMappings).
		Request(schema.ComponentTemplateFeedsMappings()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if _, err := c.API.Cluster.PutComponentTemplate(schema.FeedsSettings).
		Request(schema.ComponentTemplateFeedsSettings()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if _, err := c.API.Indices.PutIndexTemplate(schema.FeedsSchemaPrefix).
		Request(schema.IndexTemplateFeeds()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	return nil
}

// migrateFeedItems contains migration actions for migrating feed items
// index mappings & settings and ILM policy.
func (c *Client) migrateFeedItems(ctx context.Context) error {
	c.Logger.Debug("Migrating feed items...")

	if _, err := c.API.Ilm.PutLifecycle(schema.FeedItemsSchemaPrefix).
		Request(schema.ILMPolicyFeedItems()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if _, err := c.API.Cluster.PutComponentTemplate(schema.FeedsItemsMappings).
		Request(schema.ComponentTemplateFeedItemsMappings()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if _, err := c.API.Cluster.PutComponentTemplate(schema.FeedsItemsSettings).
		Request(schema.ComponentTemplateFeedItemsSettings()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if _, err := c.API.Indices.PutIndexTemplate(schema.FeedItemsSchemaPrefix).
		Request(schema.IndexTemplateFeedItems()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	return nil
}

// migrateReadItems contains migration actions for migrating read items
// index mappings & settings and ILM policy.
func (c *Client) migratedReadItems(ctx context.Context) error {
	c.Logger.Debug("Migrating read items...")

	if _, err := c.API.Ilm.PutLifecycle(schema.ReadItemsSchemaPrefix).
		Request(schema.ILMPolicyReadItems()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if _, err := c.API.Cluster.PutComponentTemplate(schema.ReadItemsMappings).
		Request(schema.ComponentTemplateReadItemsMappings()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if _, err := c.API.Cluster.PutComponentTemplate(schema.ReadItemsSettings).
		Request(schema.ComponentTemplateReadItemsSettings()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if _, err := c.API.Indices.PutIndexTemplate(schema.ReadItemsSchemaPrefix).
		Request(schema.IndexTemplateReadItems()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	return nil
}

// migrateIngest will migrate the ingest pipelines.
func (c *Client) migrateIngest(ctx context.Context) error {
	c.Logger.Debug("Migrating ingest pipeline...")

	if _, err := c.API.Ingest.PutPipeline(schema.IngestPipelineID).
		Request(schema.IngestPipelineFeeds()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	return nil
}
