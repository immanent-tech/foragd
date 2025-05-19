// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"log/slog"

	slogctx "github.com/veqryn/slog-context"
)

const (
	ThemeSessionKey = "theme"
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
