// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"encoding/gob"
	"net/url"
	"strings"

	slogctx "github.com/veqryn/slog-context"
)

func init() {
	gob.Register(Filters{})
}

const (
	ThemeSessionKey = "theme"
)

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

func (s PageState) GetFilters() Filters {
	filters := NewFilters()
	if s.Params.Has(ParamCategories) {
		filters.Categories = strings.Split(s.Params.Get(ParamCategories), ",")
	}
	filters.Count = s.Params.Get(ParamCount)
	filters.View = View(s.Params.Get(ParamView))
	filters.SortBy = SortBy(s.Params.Get(ParamSortBy))
	filters.SortOrder = SortOrder(s.Params.Get(ParamSortOrder))
	return *filters
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
