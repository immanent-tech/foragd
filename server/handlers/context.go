// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/views"
)

const (
	templateCtxKey contextKey = "templates"
	htmxRespCtxKey contextKey = "htmxResp"
)

// templateToCtx pushes (appends) the given content templates to the existing content templates stored in the
// context. If no existing templates have been stored, a new slice of templates will be created.
func templateToCtx(ctx context.Context, template templ.Component) context.Context {
	return context.WithValue(ctx, templateCtxKey, template)
}

// templateFromCtx retrieves the existing content templates from the context. If no templates are stored, an empty
// slice is returned.
func templateFromCtx(ctx context.Context) templ.Component {
	if template, ok := ctx.Value(templateCtxKey).(templ.Component); ok {
		return template
	}
	return nil
}

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
	return views.DefaultPageTitle
}
