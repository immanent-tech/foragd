// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"log/slog"

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

func GetViewFromSession(ctx context.Context, api SessionAPI, key PageViewID) PageView {
	view, ok := api.Get(ctx, "Page:"+string(key)).(PageView)
	if !ok {
		slogctx.FromCtx(ctx).Warn("No saved view found, generating defaults.",
			slog.String("view", string(key)))
		return NewPageView(key, nil)
	} else {
		return view
	}
}

func SaveViewInSession(ctx context.Context, api SessionAPI, view PageView) {
	api.Put(ctx, "Page:"+string(view.ID), view)
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
		return NewPageView(PageViewIDHome, nil)
	} else {
		return view
	}
}
