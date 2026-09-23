// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package templates

import (
	"context"
	"net/url"
)

const (
	fragmentsCtxKey contextKey = "fragments"
)

type contextKey string

const (
	NavHome          Navigation = "home"
	NavSubscriptions Navigation = "subscriptions"
	NavArticles      Navigation = "articles"
	NavMap           Navigation = "map"
	NavFavorites     Navigation = "favorites"
	NavDiscover      Navigation = "discover"
	NavSettings      Navigation = "settings"
)

// Navigation reflects what navigation marker the current page should be placed under.
type Navigation string

func FragmentKeysToCtx(ctx context.Context, keys ...templFragmentKey) context.Context {
	if len(keys) > 0 {
		return context.WithValue(ctx, fragmentsCtxKey, keys)
	}
	return ctx
}

func FragmentKeysFromCtx(ctx context.Context) []templFragmentKey {
	keys, found := ctx.Value(fragmentsCtxKey).([]templFragmentKey)
	if !found {
		return nil
	}
	return keys
}

type AppConfig interface {
	GetAppID() string
	GetAppVersion() string
	GetAppName() string
	IsProduction() bool
	GetBaseURL() *url.URL
}

type Breadcrumbs interface {
	Previous(ctx context.Context) (*url.URL, bool)
}

type SessionManager interface {
	Get(ctx context.Context, key string) any
}
