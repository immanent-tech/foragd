// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
)

// Error represents any kind of error.
type Error struct {
	template templ.Component
}

// InternalError represents errors shown on internal (pages accessible to logged in users) pages.
type InternalError Error

// HandleInternalError handles display errors on internal pages (pages accessible to logged in users).
func HandleInternalError(err error) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		var apiErr *models.APIError
		if errors.As(err, &apiErr) {
			user, _ := models.UserFromCtx(req.Context())
			apiErr.WriteLog(req.Context())
			res.WriteHeader(apiErr.HTTPStatus())
			page := &InternalError{
				template: templates.InternalError(user, apiErr.GetUserMessage()),
			}
			RenderPage(page).ServeHTTP(res, req)
		} else {
			slogctx.FromCtx(req.Context()).Error("Unknown error occurred.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

// FullResponse renders the error message on a full page.
func (p *InternalError) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(templates.CreatePage(p.template)).ServeHTTP(res, req)
}

// PartialResponse renders the error message in the content area.
func (p *InternalError) PartialResponse(res http.ResponseWriter, req *http.Request) {
	res.Header().Set(htmx.HeaderRetarget, templates.ContentID.Target())
	templ.Handler(p.template, templ.WithFragments(templates.ErrorFragment)).ServeHTTP(res, req)
}

// ExternalError represents errors shown on external (public) pages.
type ExternalError Error

// HandleExternalError handles display errors on external pages.
func HandleExternalError(err error) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		var apiErr *models.APIError
		if errors.As(err, &apiErr) {
			user, _ := models.UserFromCtx(req.Context())
			apiErr.WriteLog(req.Context())
			res.WriteHeader(apiErr.HTTPStatus())
			page := &InternalError{
				template: templates.ExternalError(user, apiErr.GetUserMessage()),
			}
			RenderPage(page).ServeHTTP(res, req)
		} else {
			slogctx.FromCtx(req.Context()).Error("Unknown error occurred.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

// FullResponse renders the error on a full page.
func (p *ExternalError) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(templates.CreatePage(p.template)).ServeHTTP(res, req)
}

// PartialResponse renders the error message itself.
func (p *ExternalError) PartialResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(p.template, templ.WithFragments(templates.ErrorFragment)).ServeHTTP(res, req)
}
