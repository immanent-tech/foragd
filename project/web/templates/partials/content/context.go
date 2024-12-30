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
	// Parent is the page to which this page should redirect back to.
	Parent string
	// Return is the page which any children should redirect back to.
	Return string
	// Pagination is a value used by the backend to fetch the next set of
	// results as the user scrolls through feeds/items.
	Pagination string
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
