// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"

	"github.com/a-h/templ"

	"github.com/joshuar/go-feed-me/models"
)

const (
	subscriptionsCtxKey contextKey = "subscriptions"
	articlesCtxKey      contextKey = "articles"
	paginationCtxKey    contextKey = "pagination"
)

func articlesToCtx(ctx context.Context, articles models.Articles) context.Context {
	return context.WithValue(ctx, articlesCtxKey, articles)
}

func articlesFromCtx(ctx context.Context) models.Articles {
	if articles, ok := ctx.Value(articlesCtxKey).(models.Articles); ok {
		return articles
	}
	return nil
}

func subscriptionsToCtx(ctx context.Context, subscriptions models.Subscriptions) context.Context {
	return context.WithValue(ctx, subscriptionsCtxKey, subscriptions)
}

func subscriptionsFromCtx(ctx context.Context) models.Subscriptions {
	if subscriptions, ok := ctx.Value(subscriptionsCtxKey).(models.Subscriptions); ok {
		return subscriptions
	}
	return nil
}

func paginationToCtx(ctx context.Context, pagination *models.Pagination) context.Context {
	return context.WithValue(ctx, paginationCtxKey, pagination)
}

func paginationFromCtx(ctx context.Context) *models.Pagination {
	if pagination, ok := ctx.Value(paginationCtxKey).(*models.Pagination); ok {
		return pagination
	}
	return nil
}

// pushContentToCtx pushes (appends) the given content templates to the existing content templates stored in the
// context. If no existing templates have been stored, a new slice of templates will be created.
func pushContentToCtx(ctx context.Context, content ...templ.Component) context.Context {
	templates := getContentFromCtx(ctx)
	templates = append(templates, content...)
	return context.WithValue(ctx, contentCtxKey, templates)
}

// shiftContentToCtx shifts (adds to the front) the given content templates to the existing content templates stored in
// the context. If no existing templates have been stored, a new slice of templates will be created.
func shiftContentToCtx(ctx context.Context, content ...templ.Component) context.Context {
	templates := getContentFromCtx(ctx)
	templates = append(content, templates...)
	return context.WithValue(ctx, contentCtxKey, templates)
}

// getContentFromCtx retrieves the existing content templates from the context. If no templates are stored, an empty
// slice is returned.
func getContentFromCtx(ctx context.Context) []templ.Component {
	if templates, ok := ctx.Value(contentCtxKey).([]templ.Component); ok {
		return templates
	}
	return make([]templ.Component, 0)
}
