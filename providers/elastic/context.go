// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic/schema"
)

var ErrFetchCtx = errors.New("error fetching context value")

const (
	userReadIndexCtxKey          contextKey = "users_ro"
	userWriteIndexCtxKey         contextKey = "users_rw"
	feedsReadIndexCtxKey         contextKey = "feeds_ro"
	feedsWriteIndexCtxKey        contextKey = "feeds_rw"
	itemsArchiveReadIndexCtxKey  contextKey = "archive_ro"
	itemsArchiveWriteIndexCtxKey contextKey = "archive_rw"
	itemsReadIndexCtxKey         contextKey = "items_ro"
	itemsWriteIndexCtxKey        contextKey = "items_rw"
	jobsReadIndexCtxKey          contextKey = "jobs_ro"
	jobsWriteIndexCtxKey         contextKey = "jobs_rw"
	jobStateReadIndexCtxKey      contextKey = "job_state_ro"
	jobStateWriteIndexCtxKey     contextKey = "job_state_rw"
)

type contextKey string

// SetupIndexAliases stores all the index aliases in the context.
func SetupIndexAliases(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, userReadIndexCtxKey, schema.UsersSchemaPrefix+schema.IndexReadSuffix)
	ctx = context.WithValue(ctx, userWriteIndexCtxKey, schema.UsersSchemaPrefix+schema.IndexWriteSuffix)
	ctx = context.WithValue(ctx, feedsReadIndexCtxKey, schema.FeedsSchemaPrefix+schema.IndexReadSuffix)
	ctx = context.WithValue(ctx, feedsWriteIndexCtxKey, schema.FeedsSchemaPrefix+schema.IndexWriteSuffix)
	ctx = context.WithValue(ctx, itemsArchiveReadIndexCtxKey, schema.ArticleArchiveSchemaPrefix+schema.IndexReadSuffix)
	ctx = context.WithValue(ctx, itemsArchiveWriteIndexCtxKey, schema.ArticleArchiveSchemaPrefix+schema.IndexWriteSuffix)
	ctx = context.WithValue(ctx, jobsReadIndexCtxKey, schema.SchedulerJobsPrefix+schema.IndexReadSuffix)
	ctx = context.WithValue(ctx, jobsWriteIndexCtxKey, schema.SchedulerJobsPrefix+schema.IndexWriteSuffix)
	ctx = context.WithValue(ctx, jobStateReadIndexCtxKey, schema.SchedulerStatePrefix+schema.IndexReadSuffix)
	ctx = context.WithValue(ctx, jobStateWriteIndexCtxKey, schema.SchedulerStatePrefix+schema.IndexWriteSuffix)
	ctx = context.WithValue(ctx, itemsReadIndexCtxKey, schema.ItemsSchemaPrefix+schema.IndexReadSuffix)
	ctx = context.WithValue(ctx, itemsWriteIndexCtxKey, schema.ItemsSchemaPrefix+schema.IndexWriteSuffix)
	return ctx
}

// UserReadIndexFromCtx retrieves the index alias for read operations against the user index.
func UserReadIndexFromCtx(ctx context.Context) (string, error) {
	if value, ok := ctx.Value(userReadIndexCtxKey).(string); ok {
		return value, nil
	}
	return "", models.NewAPIError(fmt.Errorf("%w: user read index name not found", ErrFetchCtx), http.StatusNotFound)
}

// UserWriteIndexFromCtx retrieves the index alias for write operations against the user index.
func UserWriteIndexFromCtx(ctx context.Context) (string, error) {
	if value, ok := ctx.Value(userWriteIndexCtxKey).(string); ok {
		return value, nil
	}
	return "", models.NewAPIError(fmt.Errorf("%w: user write index name not found", ErrFetchCtx), http.StatusNotFound)
}

// FeedsReadIndexFromCtx retrieves the index alias for read operations against the feeds index.
func FeedsReadIndexFromCtx(ctx context.Context) (string, error) {
	if value, ok := ctx.Value(feedsReadIndexCtxKey).(string); ok {
		return value, nil
	}
	return "", models.NewAPIError(fmt.Errorf("%w: feeds read index name not found", ErrFetchCtx), http.StatusNotFound)
}

// FeedsWriteIndexFromCtx retrieves the index alias for write operations against the feeds index.
func FeedsWriteIndexFromCtx(ctx context.Context) (string, error) {
	if value, ok := ctx.Value(feedsWriteIndexCtxKey).(string); ok {
		return value, nil
	}
	return "", models.NewAPIError(fmt.Errorf("%w: feeds write index name not found", ErrFetchCtx), http.StatusNotFound)
}

// ItemsArchiveReadIndexFromCtx retrieves the index alias for read operations against the articles archive index.
func ItemsArchiveReadIndexFromCtx(ctx context.Context) (string, error) {
	if value, ok := ctx.Value(itemsArchiveReadIndexCtxKey).(string); ok {
		return value, nil
	}
	return "", models.NewAPIError(fmt.Errorf("%w: items archive read index name not found", ErrFetchCtx), http.StatusNotFound)
}

// ItemsArchiveWriteIndexFromCtx retrieves the index alias for write operations against the articles archive index.
func ItemsArchiveWriteIndexFromCtx(ctx context.Context) (string, error) {
	if value, ok := ctx.Value(itemsArchiveWriteIndexCtxKey).(string); ok {
		return value, nil
	}
	return "", models.NewAPIError(fmt.Errorf("%w: items archive write index name not found", ErrFetchCtx), http.StatusNotFound)
}

// JobsReadIndexFromCtx retrieves the index alias for read operations against the scheduler jobs index.
func JobsReadIndexFromCtx(ctx context.Context) (string, error) {
	if value, ok := ctx.Value(jobsReadIndexCtxKey).(string); ok {
		return value, nil
	}
	return "", models.NewAPIError(fmt.Errorf("%w: jobs read index name not found", ErrFetchCtx), http.StatusNotFound)
}

// JobsWriteIndexFromCtx retrieves the index alias for write operations against the scheduler jobs index.
func JobsWriteIndexFromCtx(ctx context.Context) (string, error) {
	if value, ok := ctx.Value(jobsWriteIndexCtxKey).(string); ok {
		return value, nil
	}
	return "", models.NewAPIError(fmt.Errorf("%w: jobs write index name not found", ErrFetchCtx), http.StatusNotFound)
}

// JobStateReadIndexFromCtx retrieves the index alias for read operations against the scheduler job state index.
func JobStateReadIndexFromCtx(ctx context.Context) (string, error) {
	if value, ok := ctx.Value(jobStateReadIndexCtxKey).(string); ok {
		return value, nil
	}
	return "", models.NewAPIError(fmt.Errorf("%w: job state read index name not found", ErrFetchCtx), http.StatusNotFound)
}

// JobStateWriteIndexFromCtx retrieves the index alias for write operations against the scheduler job state index.
func JobStateWriteIndexFromCtx(ctx context.Context) (string, error) {
	if value, ok := ctx.Value(jobStateWriteIndexCtxKey).(string); ok {
		return value, nil
	}
	return "", models.NewAPIError(fmt.Errorf("%w: job state write index name not found", ErrFetchCtx), http.StatusNotFound)
}

// ItemsReadIndexFromCtx retrieves the index alias for read operations against the items datastream indicies.
func ItemsReadIndexFromCtx(ctx context.Context) (string, error) {
	if value, ok := ctx.Value(itemsReadIndexCtxKey).(string); ok {
		return value, nil
	}
	return "", models.NewAPIError(fmt.Errorf("%w: items read index name not found", ErrFetchCtx), http.StatusNotFound)
}

// ItemsWriteIndexFromCtx retrieves the index alias for write operations against the items datastream indicies.
func ItemsWriteIndexFromCtx(ctx context.Context) (string, error) {
	if value, ok := ctx.Value(itemsWriteIndexCtxKey).(string); ok {
		return value, nil
	}
	return "", models.NewAPIError(fmt.Errorf("%w: items write index name not found", ErrFetchCtx), http.StatusNotFound)
}
