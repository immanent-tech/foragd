// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package templates

import (
	"context"
)

const (
	pathCtxKey      contextKey = "path"
	fromPathCtxKey  contextKey = "fromPath"
	fragmentsCtxKey contextKey = "fragments"
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

// FromPathToCtx stores the path of the referring page in the context.
func FromPathToCtx(ctx context.Context, path string) context.Context {
	return context.WithValue(ctx, fromPathCtxKey, path)
}

// FromPathFromCtx retrieves the path of the referring page in the context.
func FromPathFromCtx(ctx context.Context) string {
	path, found := ctx.Value(fromPathCtxKey).(string)
	if !found {
		// Assume a from path of "/home" if none found.
		return "/home"
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
