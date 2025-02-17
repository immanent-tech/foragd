// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//nolint:dupl
package schema

import (
	"context"
	"errors"

	"github.com/elastic/go-elasticsearch/v8/typedapi/cluster/putcomponenttemplate"
	"github.com/elastic/go-elasticsearch/v8/typedapi/ilm/putlifecycle"
	"github.com/elastic/go-elasticsearch/v8/typedapi/indices/create"
	"github.com/elastic/go-elasticsearch/v8/typedapi/indices/putindextemplate"
	"github.com/elastic/go-elasticsearch/v8/typedapi/ingest/putpipeline"

	"github.com/joshuar/go-feed-me/internal/config"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic"
)

var validMigrations = []string{"feeds", "feeditems", "users", "ingest", "scheduler", "session"}

var ErrMigrationFailed = errors.New("schema migration failed")

type client interface {
	PutILM(ctx context.Context, name string, policy *putlifecycle.Request) error
	PutComponentTemplate(ctx context.Context, name string, template *putcomponenttemplate.Request) error
	PutIndexTemplate(ctx context.Context, name string, template *putindextemplate.Request) error
	PutIngestPipeline(ctx context.Context, name string, pipeline *putpipeline.Request) error
	IndexExists(ctx context.Context, index string) (bool, error)
	NewIndexRequest(name string, options ...elastic.CreateIndexOption) *create.Create
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
		case "session":
			err = migrateSession(ctx, client)
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

	if err := client.PutComponentTemplate(ctx, UsersMappings, UserMappingsTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := client.PutComponentTemplate(ctx, UsersSettings, UsersSettingsTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := client.PutIndexTemplate(ctx, UsersSchemaPrefix, UsersIndexTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	userIndex := UsersSchemaPrefix + "_" + config.Environment()
	// Check that a job queue index exists.
	found, err := client.IndexExists(ctx, userIndex)
	if err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}
	// Create a job queue index if not found.
	if !found {
		_, err = client.NewIndexRequest(userIndex).Do(ctx)
		if err != nil {
			return errors.Join(ErrMigrationFailed, err)
		}
	}

	return nil
}

// migrateFeeds contains migration actions for migrating feeds indices and
// settings.
func migrateFeeds(ctx context.Context, client client) error {
	logging.FromContext(ctx).Debug("Migrating feeds...")

	if err := client.PutComponentTemplate(ctx, FeedsMappings, FeedsMappingsTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := client.PutComponentTemplate(ctx, FeedsSettings, FeedsSettingsTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := client.PutIndexTemplate(ctx, FeedsSchemaPrefix, FeedsIndexTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	feedsIndex := FeedsSchemaPrefix + "_" + config.Environment()
	// Check that a job queue index exists.
	found, err := client.IndexExists(ctx, feedsIndex)
	if err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}
	// Create a job queue index if not found.
	if !found {
		_, err = client.NewIndexRequest(feedsIndex).Do(ctx)
		if err != nil {
			return errors.Join(ErrMigrationFailed, err)
		}
	}

	return nil
}

// migrateFeedItems contains migration actions for migrating feed items
// index mappings & settings and ILM policy.
func migrateFeedItems(ctx context.Context, client client) error {
	logging.FromContext(ctx).Debug("Migrating feed items...")

	if err := client.PutILM(ctx, FeedItemsSchemaPrefix, FeedItemsILMPolicy()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := client.PutComponentTemplate(ctx, FeedsItemsMappings, FeedItemsMappingsTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := client.PutComponentTemplate(ctx, FeedsItemsSettings, FeedItemsSettingsTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := client.PutIndexTemplate(ctx, FeedItemsSchemaPrefix, FeedItemsIndexTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	return nil
}

// migrateScheduler contains migration actions for migrating scheduler
// index mappings & settings and ILM policy.
func migrateScheduler(ctx context.Context, client client) error {
	logging.FromContext(ctx).Debug("Migrating feed items...")

	var (
		err   error
		found bool
	)

	// scheduler jobs indicies

	if err = client.PutComponentTemplate(ctx, SchedulerJobsMappings, SchedulerJobsMappingsTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err = client.PutComponentTemplate(ctx, SchedulerJobsSettings, SchedulerJobsSettingsTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err = client.PutIndexTemplate(ctx, SchedulerJobsPrefix, SchedulerJobsIndexTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	jobsStateIndex := SchedulerJobsPrefix + "_" + config.Environment()
	// Check that a job queue index exists.
	found, err = client.IndexExists(ctx, jobsStateIndex)
	if err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}
	// Create a job queue index if not found.
	if !found {
		_, err = client.NewIndexRequest(jobsStateIndex).Do(ctx)
		if err != nil {
			return errors.Join(ErrMigrationFailed, err)
		}
	}

	return nil
}

// migrateSession contains migration actions for migrating session indices and
// settings.
func migrateSession(ctx context.Context, client client) error {
	logging.FromContext(ctx).Debug("Migrating feeds...")

	if err := client.PutComponentTemplate(ctx, SessionsMappings, SessionsMappingsTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := client.PutComponentTemplate(ctx, SessionsSettings, SessionsSettingsTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := client.PutIndexTemplate(ctx, SessionsPrefix, SessionsIndexTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	sessionIndex := SessionsPrefix + "_" + config.Environment()
	// Check that a job queue index exists.
	found, err := client.IndexExists(ctx, sessionIndex)
	if err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}
	// Create a job queue index if not found.
	if !found {
		_, err = client.NewIndexRequest(sessionIndex).Do(ctx)
		if err != nil {
			return errors.Join(ErrMigrationFailed, err)
		}
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
