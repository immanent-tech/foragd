// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"net/url"
	"time"

	"github.com/davecgh/go-spew/spew"
	slogctx "github.com/veqryn/slog-context"
)

const (
	ThemeSessionKey       = "theme"
	PageHistorySessionKey = "page_history"
)

type SessionAPI interface {
	Put(ctx context.Context, key string, value any)
	Get(ctx context.Context, key string) any
	Commit(ctx context.Context) (string, time.Time, error)
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

func SavePageHistory(ctx context.Context, api SessionAPI, state PageState) {
	history, ok := api.Get(ctx, "bar").(*List[PageState])
	if !ok {
		slogctx.FromCtx(ctx).Warn("creating new history")
		history = &List[PageState]{}
	}
	history.Push(state)
	api.Put(ctx, "foo", history)
	spew.Dump(api.Commit(ctx))
}

func GetPageHistory(ctx context.Context, api SessionAPI) *List[PageState] {
	history, ok := api.Get(ctx, "bar").(*List[PageState])
	spew.Dump(history, ok)
	if !ok {
		slogctx.FromCtx(ctx).Warn("No history found.")
		return &List[PageState]{}
	}
	return history
}

func GetPreviousViewedPage(ctx context.Context, api SessionAPI) PageState {
	history := GetPageHistory(ctx, api)
	if history.Head.Next != nil {
		return history.Head.Next.Val
	}
	return PageState{Path: "/home"}
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
