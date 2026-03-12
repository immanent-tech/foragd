// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package templates

import (
	"context"
	"encoding/json"
)

const (
	pathCtxKey contextKey = "path"
)

type contextKey string

func generateHXVals(values map[string]any) string {
	data, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return string(data)
}

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
