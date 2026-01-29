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

// InternalError represents errors shown on internal (pages accessible to logged in users) pages.
type InternalError struct {
	err *models.APIError
}

// HandleInternalError handles display errors on internal pages (pages accessible to logged in users).
func HandleInternalError(err error) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		var apiErr *models.APIError
		if errors.As(err, &apiErr) {
			apiErr.WriteLog(req.Context())
			res.WriteHeader(apiErr.HTTPStatus())
			page := &InternalError{
				err: apiErr,
			}
			RenderInternalPage(page).ServeHTTP(res, req)
		} else {
			apiErr = &models.APIError{
				StatusCode:    http.StatusInternalServerError,
				InternalError: err,
				UserMessage: models.NewErrorMessage(
					"A backend error occurred",
					"This might be temporary, please try again.",
				),
			}
			apiErr.WriteLog(req.Context())
			res.WriteHeader(apiErr.HTTPStatus())
			page := &InternalError{
				err: apiErr,
			}
			RenderInternalPage(page).ServeHTTP(res, req)
		}
	}
}

// FullResponse renders the error message on a full page.
func (p *InternalError) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(templates.CreatePage(templates.InternalError(models.UserFromCtx(req.Context()), p.err.UserMessage))).
		ServeHTTP(res, req)
}

// PartialResponse will render the error in the content area for GET requests, as a notification otherwise.
func (p *InternalError) PartialResponse(res http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodGet {
		res.Header().Set(htmx.HeaderRetarget, templates.ContentID.Target())
		templ.Handler(templates.InternalError(models.UserFromCtx(req.Context()), p.err.UserMessage), templ.WithFragments(templates.ErrorFragment)).
			ServeHTTP(res, req)
	} else {
		res.Header().Set(htmx.HeaderReswap, "none")
		RenderPartial(&Notification{
			msg: p.err.UserMessage,
		}).ServeHTTP(res, req)
	}
}

// ExternalError represents errors shown on external (public) pages.
type ExternalError struct {
	template templ.Component
}

// HandleExternalError handles display errors on external pages.
func HandleExternalError(err error) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		var apiErr *models.APIError
		if errors.As(err, &apiErr) {
			apiErr.WriteLog(req.Context())
			res.WriteHeader(apiErr.HTTPStatus())
			page := &ExternalError{
				template: templates.ExternalError(models.UserFromCtx(req.Context()), apiErr.GetUserMessage()),
			}
			RenderExternalPage(page).ServeHTTP(res, req)
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
