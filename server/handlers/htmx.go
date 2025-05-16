// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"log/slog"
	"net/http"
	"slices"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	slogctx "github.com/veqryn/slog-context"
)

// HTMXResponse will return a handler that will render the given templates via a htmx response.
func HTMXResponse(templates ...templ.Component) http.Handler {
	return http.HandlerFunc(
		func(res http.ResponseWriter, req *http.Request) {
			resp, found := req.Context().Value(htmxRespCtxKey).(htmx.Response)
			if !found {
				slogctx.FromCtx(req.Context()).Warn("No existing htmx response object, creating new one.")
				resp = htmx.NewResponse()
			}
			for template := range slices.Values(templates) {
				if err := resp.RenderTempl(req.Context(), res, template); err != nil {
					slogctx.FromCtx(req.Context()).Warn("Template failed to render.", slog.Any("error", err))
				}
			}
		})
}

func RenderPartials(templates ...templ.Component) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			if !htmx.IsHTMX(req) {
				slogctx.FromCtx(req.Context()).Warn("Cannot render partials, not a htmx request.")
			} else {
				resp, found := req.Context().Value(htmxRespCtxKey).(htmx.Response)
				if !found {
					slogctx.FromCtx(req.Context()).Warn("No existing htmx response object, creating new one.")
					resp = htmx.NewResponse()
				}
				for template := range slices.Values(templates) {
					if err := resp.RenderTempl(req.Context(), res, template); err != nil {
						slogctx.FromCtx(req.Context()).Warn("Template failed to render.", slog.Any("error", err))
					}
				}
			}
			next.ServeHTTP(res, req)
		})
	}
}
