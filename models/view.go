// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"github.com/joshuar/go-feed-me/web/templates/action"
)

type PageView struct {
	ID      PageViewID
	Path    string
	Filters Filters
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

func NewPageView(id PageViewID, filters *Filters) PageView {
	if filters == nil {
		filters = NewFilters()
	}
	switch id {
	case PageViewIDShowItems:
		return PageView{ID: id, Path: "/home/show/items", Filters: *filters}
	case PageViewIDShowFeeds:
		return PageView{ID: id, Path: "/home/show/feeds", Filters: *filters}
	default:
		return PageView{ID: PageViewIDHome, Path: "/home"}
	}
}
