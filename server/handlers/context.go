// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"

	"github.com/angelofallars/htmx-go"
)

const (
	templateCtxKey contextKey = "templates"
	htmxRespCtxKey contextKey = "htmxResp"
)

// htmxRespToCtx adds the given htmx.Response object to the context.
func htmxRespToCtx(ctx context.Context, resp htmx.Response) context.Context {
	return context.WithValue(ctx, htmxRespCtxKey, resp)
}

// htmxRespFromCtx retrieves a htmx.Response object from the context. If none is found, it returns a new object.
func htmxRespFromCtx(ctx context.Context) htmx.Response {
	if resp, ok := ctx.Value(htmxRespCtxKey).(htmx.Response); ok {
		return resp
	}
	return htmx.NewResponse()
}

// pageTitleToCtx stores the page title in the context.
func pageTitleToCtx(ctx context.Context, title string) context.Context {
	return context.WithValue(ctx, titleCtxKey, title)
}

// pageTitleFromCtx retrieves the page title from the context.
func pageTitleFromCtx(ctx context.Context) string {
	if title, ok := ctx.Value(titleCtxKey).(string); ok {
		return title
	}
	return ""
}
