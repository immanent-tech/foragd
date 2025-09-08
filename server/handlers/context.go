// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"

	"github.com/immanent-tech/go-feed-me/models"
)

const (
	htmxRespCtxKey            contextKey = "htmxResp"
	subscriptionFiltersCtxKey contextKey = "subscriptionFilters"
	articleFiltersCtxKey      contextKey = "articleFilters"
	templateCtxKey            contextKey = "template"
)

// htmxRespToCtx adds the given htmx.Response object to the context.
func htmxRespToCtx(ctx context.Context, resp htmx.Response) context.Context {
	return context.WithValue(ctx, htmxRespCtxKey, resp)
}

// htmxRespFromCtx retrieves a htmx.Response object from the context. If none is found, it returns a new object.
func htmxRespFromCtx(ctx context.Context) htmx.Response {
	if resp, ok := ctx.Value(htmxRespCtxKey).(htmx.Response); ok {
		return resp
	}
	return htmx.NewResponse()
}

func templateToCtx(ctx context.Context, template templ.Component) context.Context {
	return context.WithValue(ctx, templateCtxKey, template)
}

func templateFromCtx(ctx context.Context) templ.Component {
	if template, ok := ctx.Value(templateCtxKey).(templ.Component); ok {
		return template
	}
	return nil
}

func subscriptionFiltersToCtx(ctx context.Context, filters models.SubscriptionFilters) context.Context {
	return context.WithValue(ctx, subscriptionFiltersCtxKey, filters)
}

func subscriptionFiltersFromCtx(ctx context.Context) models.SubscriptionFilters {
	if filters, ok := ctx.Value(subscriptionFiltersCtxKey).(models.SubscriptionFilters); ok {
		return filters
	}
	return models.NewSubscriptionFilters()
}

func articleFiltersToCtx(ctx context.Context, filters models.ArticleFilters) context.Context {
	return context.WithValue(ctx, articleFiltersCtxKey, filters)
}

func articleFiltersFromCtx(ctx context.Context) models.ArticleFilters {
	if filters, ok := ctx.Value(articleFiltersCtxKey).(models.ArticleFilters); ok {
		return filters
	}
	return models.NewArticleFilters()
}
