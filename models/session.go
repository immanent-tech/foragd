// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"log/slog"
	"net/url"

	slogctx "github.com/veqryn/slog-context"
)

const (
	ThemeSessionKey = "theme"
)

type PageView struct {
	Path    string
	Filters Filters
}

type SessionAPI interface {
	Put(ctx context.Context, key string, value any)
	Get(ctx context.Context, key string) any
}

type PageState struct {
	Path   string
	Params url.Values
}

func (s PageState) String() string {
	if len(s.Params) > 0 {
		return s.Path + "?" + s.Params.Encode()
	}
	return s.Path
}

func PageStateToSession(ctx context.Context, api SessionAPI, state PageState) {
	api.Put(ctx, "State:"+state.Path, state)
}

func PageStateFromSession(ctx context.Context, api SessionAPI, path string) PageState {
	state, ok := api.Get(ctx, "State:"+path).(PageState)
	if !ok {
		slogctx.FromCtx(ctx).Warn("No saved page state found.",
			slog.String("path", path))
		return PageState{Path: path}
	}
	return state
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

func BacklinkToCtx(ctx context.Context, backlink PageState) context.Context {
	return context.WithValue(ctx, backlinkCtxKey, backlink)
}

func BacklinkFromCtx(ctx context.Context) PageState {
	backlink, found := ctx.Value(backlinkCtxKey).(PageState)
	if !found {
		return PageState{Path: "/home"}
	}
	return backlink
}
