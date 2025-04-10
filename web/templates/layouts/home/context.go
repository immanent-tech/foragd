// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"context"

	"github.com/joshuar/go-feed-me/internal/models"
)

const (
	contentCtxKey contextKey = "content"
	footerCtxKey  contextKey = "footer"
	titleCtxKey   contextKey = "title"
)

type contextKey string

// FiltersToCtx stores the current filters in the context.
func ContentToCtx(ctx context.Context, content []models.Template) context.Context {
	return context.WithValue(ctx, contentCtxKey, content)
}

// FiltersFromCtx retrieves the current filters from the context. If there are no filters stored in the context, the
// default filters will be returned.
func ContentFromCtx(ctx context.Context) []models.Template {
	content, found := ctx.Value(contentCtxKey).([]models.Template)
	if !found {
		return nil
	}
	return content
}

func FooterToCtx(ctx context.Context, footer models.Template) context.Context {
	return context.WithValue(ctx, footerCtxKey, footer)
}

// FiltersFromCtx retrieves the current filters from the context. If there are no filters stored in the context, the
// default filters will be returned.
func FooterFromCtx(ctx context.Context) models.Template {
	footer, found := ctx.Value(footerCtxKey).(models.Template)
	if !found {
		return nil
	}
	return footer
}

func TitleToCtx(ctx context.Context, title string) context.Context {
	return context.WithValue(ctx, titleCtxKey, title)
}

// FiltersFromCtx retrieves the current filters from the context. If there are no filters stored in the context, the
// default filters will be returned.
func TitleFromCtx(ctx context.Context) string {
	title, found := ctx.Value(footerCtxKey).(string)
	if !found {
		return "Home"
	}
	return title
}
