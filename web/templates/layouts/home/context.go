// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"context"

	"github.com/a-h/templ"
)

const (
	contentCtxKey contextKey = "content"
	titleCtxKey   contextKey = "title"
)

type contextKey string

// FiltersToCtx stores the current filters in the context.
func ContentToCtx(ctx context.Context, content []templ.Component) context.Context {
	return context.WithValue(ctx, contentCtxKey, content)
}

// FiltersFromCtx retrieves the current filters from the context. If there are no filters stored in the context, the
// default filters will be returned.
func ContentFromCtx(ctx context.Context) []templ.Component {
	content, found := ctx.Value(contentCtxKey).([]templ.Component)
	if !found {
		return nil
	}
	return content
}

func TitleToCtx(ctx context.Context, title string) context.Context {
	return context.WithValue(ctx, titleCtxKey, title)
}

// FiltersFromCtx retrieves the current filters from the context. If there are no filters stored in the context, the
// default filters will be returned.
func TitleFromCtx(ctx context.Context) string {
	title, found := ctx.Value(titleCtxKey).(string)
	if !found {
		return "Home"
	}
	return title
}
