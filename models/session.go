// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"log/slog"
	"net/url"

	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/web/templates/action"
)

const (
	ThemeSessionKey        = "theme"
	lastPageViewSessionKey = "last_viewed"
)

type PageView struct {
	Path    string
	Filters Filters
}

func NewPageView(path string, filters *Filters) PageView {
	if filters == nil {
		filters = NewFilters()
	}
	return PageView{Path: path, Filters: *filters}
}

func (r PageView) String() string {
	return r.Path + "?" + r.Filters.ToQueryParams().Encode()
}

func (r PageView) AsAction(options ...action.Option) *action.Action {
	viewAction := action.Build(r.Path, options...)
	// Set the query params from the filters.
	action.WithParams(r.Filters.ToQueryParams())(viewAction)
	return viewAction
}

type SessionAPI interface {
	Put(ctx context.Context, key string, value any)
	Get(ctx context.Context, key string) any
}

func GetViewFromSession(ctx context.Context, api SessionAPI, key PageViewID) PageView {
	view, ok := api.Get(ctx, "State:"+string(key)).(PageView)
	if !ok {
		slogctx.FromCtx(ctx).Warn("No saved view found, generating defaults.",
			slog.String("view", string(key)))
		return NewPageView("/home", nil)
	} else {
		return view
	}
}

func SaveViewInSession(ctx context.Context, api SessionAPI, key PageViewID, view PageView) {
	api.Put(ctx, "State:"+string(key), view)
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

func SavePageStateInSession(ctx context.Context, api SessionAPI, state PageState) {
	api.Put(ctx, "State:"+state.Path, state)
}

func RestorePageStateFromSession(ctx context.Context, api SessionAPI, path string) PageState {
	state, ok := api.Get(ctx, "State:"+path).(PageState)
	if !ok {
		slogctx.FromCtx(ctx).Warn("No saved page state found.",
			slog.String("path", path))
		return PageState{Path: path}
	}
	return state
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

func ViewToCtx(ctx context.Context, view PageView) context.Context {
	return context.WithValue(ctx, pageViewStateCtxKey, view)
}

func ViewFromCtx(ctx context.Context) PageView {
	view, found := ctx.Value(pageViewStateCtxKey).(PageView)
	if !found {
		slogctx.FromCtx(ctx).Warn("No view found in context, using default.")
		return NewPageView("/home", nil)
	}
	return view
}
