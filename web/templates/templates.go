// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package templates

import (
	"context"
	"net/url"
)

const (
	pathCtxKey      contextKey = "path"
	fragmentsCtxKey contextKey = "fragments"
	fontStyleCtxKey contextKey = "fontStyle"
	themeCtxKey     contextKey = "theme"
	baseURLCtxKey   contextKey = "baseURL"
)

type contextKey string

// PathToCtx stores the URL path in the context.
func PathToCtx(ctx context.Context, path string) context.Context {
	return context.WithValue(ctx, pathCtxKey, path)
}

// PathFromCtx retrieves the URL path from the context.
func PathFromCtx(ctx context.Context) string {
	path, found := ctx.Value(pathCtxKey).(string)
	if !found {
		return ""
	}
	return path
}

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

func FontStyleToCtx(ctx context.Context, style string) context.Context {
	return context.WithValue(ctx, fontStyleCtxKey, style)
}

func FontStyleFromCtx(ctx context.Context) string {
	style, found := ctx.Value(fontStyleCtxKey).(string)
	if !found {
		return "--font-oldstyle"
	}
	return style
}

func ThemeToCtx(ctx context.Context, theme string) context.Context {
	return context.WithValue(ctx, themeCtxKey, theme)
}

func ThemeFromCtx(ctx context.Context) string {
	theme, found := ctx.Value(themeCtxKey).(string)
	if !found {
		return "greenhouse"
	}
	return theme
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
