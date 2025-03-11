// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//nolint:dupl
package schema

import (
	"context"
	"errors"

	"github.com/elastic/go-elasticsearch/v8/typedapi"
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
func Migration(ctx context.Context, api *typedapi.API, migrations ...string) error {
	// If no migrations are specified, perform migrations for all items.
	if len(migrations) == 0 {
		migrations = validMigrations
	}

	// Perform requested migrations.
	for _, migration := range migrations {
		var err error

		switch migration {
		case "users":
			err = migrateUsers(ctx, api)
		case "feeds":
			err = migrateFeeds(ctx, api)
		case "feeditems":
			err = migrateFeedItems(ctx, api)
		case "scheduler":
			err = migrateScheduler(ctx, api)
		case "session":
			err = migrateSession(ctx, api)
		case "ingest":
			err = migrateIngest(ctx, api)
		}

		if err != nil {
			return errors.Join(ErrMigrationFailed, err)
		}
	}

	return nil
}

// migrateUsers contains migration actions for migrating users indices and
// settings.
func migrateUsers(ctx context.Context, api *typedapi.API) error {
	logging.FromContext(ctx).Debug("Migrating users...")

	if err := elastic.PutComponentTemplate(ctx, api, UsersMappings, UserMappingsTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := elastic.PutComponentTemplate(ctx, api, UsersSettings, UsersSettingsTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := elastic.PutIndexTemplate(ctx, api, UsersSchemaPrefix, UsersIndexTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	userIndex := UsersSchemaPrefix + "_" + config.Environment()
	// Check that a job queue index exists.
	found, err := elastic.IndexExists(ctx, api, userIndex)
	if err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}
	// Create a job queue index if not found.
	if !found {
		_, err = elastic.NewIndexRequest(api, userIndex).Do(ctx)
		if err != nil {
			return errors.Join(ErrMigrationFailed, err)
		}
	}

	return nil
}

// migrateFeeds contains migration actions for migrating feeds indices and
// settings.
func migrateFeeds(ctx context.Context, api *typedapi.API) error {
	logging.FromContext(ctx).Debug("Migrating feeds...")

	if err := elastic.PutComponentTemplate(ctx, api, FeedsMappings, FeedsMappingsTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := elastic.PutComponentTemplate(ctx, api, FeedsSettings, FeedsSettingsTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := elastic.PutIndexTemplate(ctx, api, FeedsSchemaPrefix, FeedsIndexTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	feedsIndex := FeedsSchemaPrefix + "_" + config.Environment()
	// Check that a job queue index exists.
	found, err := elastic.IndexExists(ctx, api, feedsIndex)
	if err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}
	// Create a job queue index if not found.
	if !found {
		_, err = elastic.NewIndexRequest(api, feedsIndex).Do(ctx)
		if err != nil {
			return errors.Join(ErrMigrationFailed, err)
		}
	}

	return nil
}

// migrateFeedItems contains migration actions for migrating feed items
// index mappings & settings and ILM policy.
func migrateFeedItems(ctx context.Context, api *typedapi.API) error {
	logging.FromContext(ctx).Debug("Migrating feed items...")

	if err := elastic.PutILM(ctx, api, FeedItemsSchemaPrefix, FeedItemsILMPolicy()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := elastic.PutComponentTemplate(ctx, api, FeedsItemsMappings, FeedItemsMappingsTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := elastic.PutComponentTemplate(ctx, api, FeedsItemsSettings, FeedItemsSettingsTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := elastic.PutIndexTemplate(ctx, api, FeedItemsSchemaPrefix, FeedItemsIndexTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	return nil
}

// migrateScheduler contains migration actions for migrating scheduler
// index mappings & settings and ILM policy.
func migrateScheduler(ctx context.Context, api *typedapi.API) error {
	logging.FromContext(ctx).Debug("Migrating feed items...")

	var (
		err   error
		found bool
	)

	// scheduler jobs indicies

	if err = elastic.PutComponentTemplate(ctx, api, SchedulerJobsMappings, SchedulerJobsMappingsTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err = elastic.PutComponentTemplate(ctx, api, SchedulerJobsSettings, SchedulerJobsSettingsTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err = elastic.PutIndexTemplate(ctx, api, SchedulerJobsPrefix, SchedulerJobsIndexTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	jobsStateIndex := SchedulerJobsPrefix + "_" + config.Environment()
	// Check that a job queue index exists.
	found, err = elastic.IndexExists(ctx, api, jobsStateIndex)
	if err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}
	// Create a job queue index if not found.
	if !found {
		_, err = elastic.NewIndexRequest(api, jobsStateIndex).Do(ctx)
		if err != nil {
			return errors.Join(ErrMigrationFailed, err)
		}
	}

	return nil
}

// migrateSession contains migration actions for migrating session indices and
// settings.
func migrateSession(ctx context.Context, api *typedapi.API) error {
	logging.FromContext(ctx).Debug("Migrating feeds...")

	if err := elastic.PutComponentTemplate(ctx, api, SessionsMappings, SessionsMappingsTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := elastic.PutComponentTemplate(ctx, api, SessionsSettings, SessionsSettingsTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	if err := elastic.PutIndexTemplate(ctx, api, SessionsPrefix, SessionsIndexTemplate()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	sessionIndex := SessionsPrefix + "_" + config.Environment()
	// Check that a job queue index exists.
	found, err := elastic.IndexExists(ctx, api, sessionIndex)
	if err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}
	// Create a job queue index if not found.
	if !found {
		_, err = elastic.NewIndexRequest(api, sessionIndex).Do(ctx)
		if err != nil {
			return errors.Join(ErrMigrationFailed, err)
		}
	}

	return nil
}

// migrateIngest will migrate the ingest pipelines.
func migrateIngest(ctx context.Context, api *typedapi.API) error {
	logging.FromContext(ctx).Debug("Migrating ingest pipeline...")

	if err := elastic.PutIngestPipeline(ctx, api, IngestPipelineID, IngestPipelineFeeds()); err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	return nil
}
