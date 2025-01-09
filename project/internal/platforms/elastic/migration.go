// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//nolint:dupl
package elastic

import (
	"context"
	"errors"

	"github.com/joshuar/go-feed-me/internal/platforms/elastic/schema"
)

var validMigrations = []string{"feeds", "feeditems", "users", "ingest"}

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
		case "users":
			err = c.migrateUsers(ctx)
		case "feeds":
			err = c.migrateFeeds(ctx)
		case "feeditems":
			err = c.migrateFeedItems(ctx)
		case "ingest":
			err = c.migrateIngest(ctx)
		}

		if err != nil {
			return errors.Join(ErrMigrationFailed, err)
		}
	}

	return nil
}

// migrateUsers contains migration actions for migrating users indices and
// settings.
func (c *Client) migrateUsers(ctx context.Context) error {
	c.Logger.Debug("Migrating users...")

	if _, err := c.API.Cluster.PutComponentTemplate(schema.UsersMappings).
		Request(schema.ComponentTemplateUserMappings()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if _, err := c.API.Cluster.PutComponentTemplate(schema.UsersSettings).
		Request(schema.ComponentTemplateUsersSettings()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if _, err := c.API.Indices.PutIndexTemplate(schema.UsersSchemaPrefix).
		Request(schema.IndexTemplateUsers()).Do(ctx); err != nil {
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

// migrateIngest will migrate the ingest pipelines.
func (c *Client) migrateIngest(ctx context.Context) error {
	c.Logger.Debug("Migrating ingest pipeline...")

	if _, err := c.API.Ingest.PutPipeline(schema.IngestPipelineID).
		Request(schema.IngestPipelineFeeds()).Do(ctx); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	return nil
}
