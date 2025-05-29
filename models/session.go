// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"net/url"

	slogctx "github.com/veqryn/slog-context"
)

const (
	ThemeSessionKey       = "theme"
	PageHistorySessionKey = "page_history"
)

type PageState struct {
	Path   string     `json:"path"`
	Params url.Values `json:"params"`
}

func (s PageState) String() string {
	if len(s.Params) > 0 {
		return s.Path + "?" + s.Params.Encode()
	}
	return s.Path
}

func PageStateToCtx(ctx context.Context, view PageState) context.Context {
	return context.WithValue(ctx, pageViewStateCtxKey, view)
}

func PageStateFromCtx(ctx context.Context) PageState {
	view, found := ctx.Value(pageViewStateCtxKey).(PageState)
	if !found {
		slogctx.FromCtx(ctx).Warn("No view found in context, using default.")
		return PageState{Path: "/home"}
	}
	return view
}
