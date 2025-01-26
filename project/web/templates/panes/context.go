// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package panes

import (
	"context"
	"errors"

	"github.com/joshuar/go-feed-me/internal/models"
)

type contextKey string

const (
	navigationCtxKey contextKey = "navigation"
)

var ErrNotInCtx = errors.New("not found in context")

// NavigationLinks contains the links for navigating between pages.
type NavigationLinks struct {
	// Parent is the base URL back links should use.
	Parent string
	// ParentFilters are the filters applied to the parent page.
	ParentFilters *models.APIFilters
	// Current is the base URL of the current page.
	Current string
	// CurrentFilters are the filters applied to the current page.
	CurrentFilters *models.APIFilters
	// Action is the base URL for actions.
	Action string
}

func (n NavigationLinks) BackLink() string {
	if n.Parent != "" {
		return n.Parent
	}

	return ""
}

func (n NavigationLinks) RefreshLink() string {
	if n.Current == "" {
		return ""
	}

	if n.CurrentFilters == nil {
		return n.Current
	}

	link, _ := n.CurrentFilters.GenerateURL(n.Current)

	return link
}

func (n NavigationLinks) MarkAllReadLink() string {
	if n.Action == "" {
		return ""
	}

	if n.CurrentFilters == nil {
		return n.Action
	}

	link, _ := n.CurrentFilters.GenerateURL(n.Action)

	return link
}

func NavigationToCtx(ctx context.Context, nav NavigationLinks) context.Context {
	return context.WithValue(ctx, navigationCtxKey, nav)
}

func NavigationFromCtx(ctx context.Context) NavigationLinks {
	nav, ok := ctx.Value(navigationCtxKey).(NavigationLinks)
	if !ok {
		return NavigationLinks{}
	}

	return nav
}
