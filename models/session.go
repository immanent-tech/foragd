// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"log/slog"
	"strings"

	slogctx "github.com/veqryn/slog-context"
)

const (
	ThemeSessionKey        = "theme"
	lastPageViewSessionKey = "last_viewed"
)

type SessionAPI interface {
	Put(ctx context.Context, key string, value any)
	Get(ctx context.Context, key string) any
}

func GetViewFromSession(ctx context.Context, api SessionAPI, path string) PageView {
	view, ok := api.Get(ctx, "View:"+path).(PageView)
	if !ok {
		slogctx.FromCtx(ctx).Warn("No save view found for path, generating defaults.",
			slog.String("path", path))
		return NewPageView(path, NewFilters())
	} else {
		return view
	}
}

func SaveViewInSession(ctx context.Context, api SessionAPI, view PageView) {
	api.Put(ctx, "View:"+view.Path, view)
}

// SaveLastPageView saves the given page view to the session.
func SaveLastPageView(ctx context.Context, api SessionAPI, view PageView) {
	api.Put(ctx, lastPageViewSessionKey, view)
}

// GetLastPageView retrieves the last page view from the session.
func GetLastPageView(ctx context.Context, api SessionAPI) PageView {
	view, ok := api.Get(ctx, lastPageViewSessionKey).(PageView)
	if !ok {
		slogctx.FromCtx(ctx).Warn("No last page view found, using default.")
		return NewPageView("/home", NewFilters())
	} else {
		return view
	}
}

// GetBacklink retrieves the appropriate PageView to use as a backlink for the given path from the stored session data.
func GetBacklink(ctx context.Context, api SessionAPI, path string) PageView {
	switch {
	case strings.Contains(path, "feed_") && strings.Contains(path, "item_"):
		return GetViewFromSession(ctx, api, ItemsRoute)
	case path == ItemsRoute:
		return GetViewFromSession(ctx, api, FeedsRoute)
	case path == FeedsRoute:
		fallthrough
	default:
		return NewPageView("/home", nil)
	}
}
