// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"

	"github.com/joshuar/go-feed-me/internal/models"
)

var (
	ErrFetchCtx = models.WrapError(
		errors.New("no index name/pattern found in context"),
		"elastic",
		"backend is not initialized properly")
)

const (
	userIndexCtxKey     contextKey = "users"
	feedsIndexCtxKey    contextKey = "feeds"
	itemsIndexCtxKey    contextKey = "items"
	subscriptionsCtxKey contextKey = "subscriptions"
	jobsIndexCtxKey     contextKey = "jobs"
	queueIndexCtxKey    contextKey = "queue"
	sessionIndexCtxKey  contextKey = "session"
	pipelineIndexCtxKey contextKey = "pipeline"
)

type contextKey string

// IndexToCtx stores an index name/pattern, used for Elasticsearch requests, in
// the given context.
func UserIndexToCtx(ctx context.Context, index string) context.Context {
	return context.WithValue(ctx, userIndexCtxKey, index)
}

// IndexFromCtx retrieves an index name/pattern, used for Elasticsearch
// requests, from the given context.
func UserIndexFromCtx(ctx context.Context) string {
	if value, ok := ctx.Value(userIndexCtxKey).(string); ok {
		return value
	}

	return ""
}

// IndexToCtx stores an index name/pattern, used for Elasticsearch requests, in
// the given context.
func JobsIndexToCtx(ctx context.Context, index string) context.Context {
	return context.WithValue(ctx, jobsIndexCtxKey, index)
}

// IndexFromCtx retrieves an index name/pattern, used for Elasticsearch
// requests, from the given context.
func JobsIndexFromCtx(ctx context.Context) string {
	if value, ok := ctx.Value(jobsIndexCtxKey).(string); ok {
		return value
	}

	return ""
}

// IndexToCtx stores an index name/pattern, used for Elasticsearch requests, in
// the given context.
func ItemsIndexToCtx(ctx context.Context, index string) context.Context {
	return context.WithValue(ctx, itemsIndexCtxKey, index)
}

// IndexFromCtx retrieves an index name/pattern, used for Elasticsearch
// requests, from the given context.
func ItemsIndexFromCtx(ctx context.Context) string {
	if value, ok := ctx.Value(itemsIndexCtxKey).(string); ok {
		return value
	}

	return ""
}

// IndexToCtx stores an index name/pattern, used for Elasticsearch requests, in
// the given context.
func FeedsIndexToCtx(ctx context.Context, index string) context.Context {
	return context.WithValue(ctx, feedsIndexCtxKey, index)
}

// IndexFromCtx retrieves an index name/pattern, used for Elasticsearch
// requests, from the given context.
func FeedsIndexFromCtx(ctx context.Context) string {
	if value, ok := ctx.Value(feedsIndexCtxKey).(string); ok {
		return value
	}

	return ""
}

func SubscriptionsIndexToCtx(ctx context.Context, index string) context.Context {
	return context.WithValue(ctx, subscriptionsCtxKey, index)
}

func SubscriptionsIndexFromCtx(ctx context.Context) string {
	if value, ok := ctx.Value(subscriptionsCtxKey).(string); ok {
		return value
	}

	return ""
}

// IndexToCtx stores an index name/pattern, used for Elasticsearch requests, in
// the given context.
func IngestPipelineToCtx(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, pipelineIndexCtxKey, name)
}

// IndexFromCtx retrieves an index name/pattern, used for Elasticsearch
// requests, from the given context.
func IngestPipelineFromCtx(ctx context.Context) string {
	if value, ok := ctx.Value(pipelineIndexCtxKey).(string); ok {
		return value
	}

	return ""
}
