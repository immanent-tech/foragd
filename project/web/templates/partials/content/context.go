// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package content

import (
	"context"
	"errors"
)

type contextKey string

const (
	navigationCtxKey contextKey = "navigation"

	HeaderBacklink = "Gofeedme-Backlink"
)

var ErrNotInCtx = errors.New("not found in context")

// NavigationLinks contains the links for navigating between pages.
type NavigationLinks struct {
	// RefreshPath is the URL to refresh the current page.
	RefreshPath string
	// BackPath is the URL to which this page should redirect back to.
	BackPath string
	// MarkReadPath is the URL to perform an action on all items on the current
	// page.
	MarkReadPath string
	// Pagination is a value used by the backend to fetch the next set of
	// results as the user scrolls through feeds/items.
	Pagination string
	// ChildActionBasePath is the base path of the URL for any actions on items
	// that the page directs to.
	ChildActionBasePath string
	// ActionBasePath is the base path of the URL for any actions on items on
	// the current page.
	ActionBasePath string
}

func NavigationToCtx(ctx context.Context, nav NavigationLinks) context.Context {
	newCtx := context.WithValue(ctx, navigationCtxKey, nav)

	return newCtx
}

func NavigationFromCtx(ctx context.Context) NavigationLinks {
	nav, ok := ctx.Value(navigationCtxKey).(NavigationLinks)
	if !ok {
		return NavigationLinks{}
	}

	return nav
}
