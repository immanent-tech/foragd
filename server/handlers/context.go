// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
)

const (
	templatesCtxKey contextKey = "templates"
	htmxRespCtxKey  contextKey = "htmxResp"
)

// pushTemplatesToCtx pushes (appends) the given content templates to the existing content templates stored in the
// context. If no existing templates have been stored, a new slice of templates will be created.
func pushTemplatesToCtx(ctx context.Context, content ...templ.Component) context.Context {
	templates := getTemplatesFromCtx(ctx)
	templates = append(templates, content...)
	return context.WithValue(ctx, templatesCtxKey, templates)
}

// shiftTemplatesToCtx shifts (adds to the front) the given content templates to the existing content templates stored in
// the context. If no existing templates have been stored, a new slice of templates will be created.
func shiftTemplatesToCtx(ctx context.Context, content ...templ.Component) context.Context {
	templates := getTemplatesFromCtx(ctx)
	templates = append(content, templates...)
	return context.WithValue(ctx, templatesCtxKey, templates)
}

// getTemplatesFromCtx retrieves the existing content templates from the context. If no templates are stored, an empty
// slice is returned.
func getTemplatesFromCtx(ctx context.Context) []templ.Component {
	if templates, ok := ctx.Value(templatesCtxKey).([]templ.Component); ok {
		return templates
	}
	return make([]templ.Component, 0)
}

// htmxRespToCtx adds the given htmx.Response object to the context.
func htmxRespToCtx(ctx context.Context, resp htmx.Response) context.Context {
	return context.WithValue(ctx, templatesCtxKey, resp)
}

// htmxRespFromCtx retrieves a htmx.Response object from the context. If none is found, it returns a new object.
func htmxRespFromCtx(ctx context.Context) htmx.Response {
	if resp, ok := ctx.Value(htmxRespCtxKey).(htmx.Response); ok {
		return resp
	}
	return htmx.NewResponse()
}
