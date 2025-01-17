// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"context"
	"errors"

	"github.com/elastic/go-elasticsearch/v8/typedapi/cluster/putcomponenttemplate"
	"github.com/elastic/go-elasticsearch/v8/typedapi/ilm/putlifecycle"
	"github.com/elastic/go-elasticsearch/v8/typedapi/indices/putindextemplate"
	"github.com/elastic/go-elasticsearch/v8/typedapi/ingest/putpipeline"

	"github.com/joshuar/go-feed-me/internal/logging"
)

var validMigrations = []string{"feeds", "feeditems", "users", "ingest", "scheduler"}

var ErrMigrationFailed = errors.New("schema migration failed")

type client interface {
	PutILM(ctx context.Context, name string, policy *putlifecycle.Request) error
	PutComponentTemplate(ctx context.Context, name string, template *putcomponenttemplate.Request) error
	PutIndexTemplate(ctx context.Context, name string, template *putindextemplate.Request) error
	PutIngestPipeline(ctx context.Context, name string, pipeline *putpipeline.Request) error
}

// Migration will create all necessary index templates settings and policies.
func Migration(ctx context.Context, client client, migrations ...string) error {
	// If no migrations are specified, perform migrations for all items.
	if len(migrations) == 0 {
		migrations = validMigrations
	}

	// Perform requested migrations.
	for _, migration := range migrations {
		var err error

		switch migration {
		case "users":
			err = migrateUsers(ctx, client)
		case "feeds":
			err = migrateFeeds(ctx, client)
		case "feeditems":
			err = migrateFeedItems(ctx, client)
		case "scheduler":
			err = migrateScheduler(ctx, client)
		case "ingest":
			err = migrateIngest(ctx, client)
		}

		if err != nil {
			return errors.Join(ErrMigrationFailed, err)
		}
	}

	return nil
}

// migrateUsers contains migration actions for migrating users indices and
// settings.
func migrateUsers(ctx context.Context, client client) error {
	logging.FromContext(ctx).Debug("Migrating users...")

	if err := client.PutComponentTemplate(ctx, UsersMappings, ComponentTemplateUserMappings()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := client.PutComponentTemplate(ctx, UsersSettings, ComponentTemplateUsersSettings()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := client.PutIndexTemplate(ctx, UsersSchemaPrefix, IndexTemplateUsers()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	return nil
}

// migrateFeeds contains migration actions for migrating feeds indices and
// settings.
func migrateFeeds(ctx context.Context, client client) error {
	logging.FromContext(ctx).Debug("Migrating feeds...")

	if err := client.PutComponentTemplate(ctx, FeedsMappings, ComponentTemplateFeedsMappings()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := client.PutComponentTemplate(ctx, FeedsSettings, ComponentTemplateFeedsSettings()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := client.PutIndexTemplate(ctx, FeedsSchemaPrefix, IndexTemplateFeeds()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	return nil
}

// migrateFeedItems contains migration actions for migrating feed items
// index mappings & settings and ILM policy.
func migrateFeedItems(ctx context.Context, client client) error {
	logging.FromContext(ctx).Debug("Migrating feed items...")

	if err := client.PutILM(ctx, FeedItemsSchemaPrefix, ILMPolicyFeedItems()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := client.PutComponentTemplate(ctx, FeedsItemsMappings, ComponentTemplateFeedItemsMappings()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := client.PutComponentTemplate(ctx, FeedsItemsSettings, ComponentTemplateFeedItemsSettings()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := client.PutIndexTemplate(ctx, FeedItemsSchemaPrefix, IndexTemplateFeedItems()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	return nil
}

// migrateScheduler contains migration actions for migrating scheduler
// index mappings & settings and ILM policy.
func migrateScheduler(ctx context.Context, client client) error {
	logging.FromContext(ctx).Debug("Migrating feed items...")

	// scheduler state indicies

	if err := client.PutComponentTemplate(ctx, SchedulerStateMappings, ComponentTemplateSchedulerStateMappings()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := client.PutComponentTemplate(ctx, SchedulerStateSettings, ComponentTemplateSchedulerStateSettings()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := client.PutIndexTemplate(ctx, SchedulerStatePrefix, IndexTemplateSchedulerState()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	// scheduler jobs indicies

	if err := client.PutComponentTemplate(ctx, SchedulerJobsMappings, ComponentTemplateSchedulerJobsMappings()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := client.PutComponentTemplate(ctx, SchedulerJobsSettings, ComponentTemplateSchedulerJobsSettings()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := client.PutIndexTemplate(ctx, SchedulerJobsPrefix, IndexTemplateSchedulerJobs()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	return nil
}

// migrateIngest will migrate the ingest pipelines.
func migrateIngest(ctx context.Context, client client) error {
	logging.FromContext(ctx).Debug("Migrating ingest pipeline...")

	if err := client.PutIngestPipeline(ctx, IngestPipelineID, IngestPipelineFeeds()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	return nil
}
