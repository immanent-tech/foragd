// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package templates

import (
	"context"
)

const (
	layoutCtxKey contextKey = "layout"
)

type contextKey string

// FiltersToCtx stores the current filters in the context.
func LayoutToCtx(ctx context.Context, content Layout) context.Context {
	return context.WithValue(ctx, layoutCtxKey, content)
}

// FiltersFromCtx retrieves the current filters from the context. If there are no filters stored in the context, the
// default filters will be returned.
func LayoutFromCtx(ctx context.Context) Layout {
	content, found := ctx.Value(layoutCtxKey).(Layout)
	if !found {
		return nil
	}
	return content
}
