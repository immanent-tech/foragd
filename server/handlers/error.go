// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"slices"

	"github.com/a-h/templ"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/pkg/htmx"

	"github.com/immanent-tech/foragd/models"
	gerror "github.com/immanent-tech/foragd/providers/google/error"
	"github.com/immanent-tech/foragd/server/otel"
	"github.com/immanent-tech/foragd/web/templates"
)

type ErrorOptions struct {
	UserMessage *models.UserMessage
}

type ErrorOption func(*ErrorOptions)

func WithUserMessage(msg *models.UserMessage) ErrorOption {
	return func(eo *ErrorOptions) {
		eo.UserMessage = msg
	}
}

// InternalError represents errors shown on internal (pages accessible to logged in users) pages.
type InternalError struct {
	err     *models.APIError
	referer string
}

// HandleInternalError handles display errors on internal pages (pages accessible to logged in users). If the passed in
// error can be unwrapped as a models.APIError, the status code and value of the APIError is used and the passed in
// status code is ignored. Otherwise a new APIError will be generated from the status code and error.
func HandleInternalError(status int, err error, options ...ErrorOption) http.HandlerFunc {
	opts := &ErrorOptions{
		UserMessage: models.NewErrorMessage(
			"Problem occurred while processing request",
			"This might be temporary, please try again.",
		),
	}
	for option := range slices.Values(options) {
		option(opts)
	}

	return func(res http.ResponseWriter, req *http.Request) {
		if otel.IsEnabled() {
			_, span := otel.TracerProvider.Tracer("").
				Start(req.Context(), "handle-internal-error")
			defer span.End()
		}

		// Don't cache errors.
		res.Header().Set("Cache-Control", "no-store")

		if apiErr, ok := errors.AsType[*models.APIError](err); ok {
			// Write appropriately leveled log message.
			apiErr.WriteLog(req.Context())
			// For 500+ errors, log to GCP error console.
			if apiErr.HTTPStatus() >= 500 {
				gerror.ReportError(req.Context(), apiErr)
			}
			// Write response.
			res.WriteHeader(apiErr.HTTPStatus())
			if apiErr.UserMessage == nil {
				// Add a generic user message if one isn't already set.
				apiErr.UserMessage = opts.UserMessage
			}
			page := &InternalError{
				err:     apiErr,
				referer: req.URL.Path,
			}
			RenderInternalPage(page).ServeHTTP(res, req)
		} else {
			if status == 0 {
				status = http.StatusInternalServerError
			}
			apiErr = &models.APIError{
				StatusCode:    status,
				InternalError: err,
				UserMessage:   opts.UserMessage,
			}
			apiErr.WriteLog(req.Context())
			res.WriteHeader(apiErr.HTTPStatus())
			page := &InternalError{
				err:     apiErr,
				referer: req.URL.Path,
			}
			RenderInternalPage(page).ServeHTTP(res, req)
		}
	}
}

// FullResponse renders the error message on a full page.
func (p *InternalError) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(templates.CreatePage(templates.InternalError(models.UserFromCtx(req.Context()), p.referer, p.err.UserMessage))).
		ServeHTTP(res, req)
}

// PartialResponse will render the error in the content area for GET requests, as a notification otherwise.
func (p *InternalError) PartialResponse(res http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodGet {
		res.Header().Set(htmx.HeaderRetarget, templates.ContentID.Target())
		templ.Handler(templates.InternalError(models.UserFromCtx(req.Context()), p.referer, p.err.UserMessage), templ.WithFragments(templates.ErrorFragment)).
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
		if otel.IsEnabled() {
			_, span := otel.TracerProvider.Tracer("").
				Start(req.Context(), "handle-external-error")
			defer span.End()
		}

		// Don't cache errors.
		res.Header().Set("Cache-Control", "no-store")

		if apiErr, ok := errors.AsType[*models.APIError](err); ok {
			// Write appropriately leveled log message.
			apiErr.WriteLog(req.Context())
			// For 500+ errors, log to GCP error console.
			if apiErr.HTTPStatus() >= 500 {
				gerror.ReportError(req.Context(), apiErr)
			}
			// Write response.
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
	templ.Handler(templates.CreatePage(p.template, templates.WithPageTitle(templates.PageTitle{Summary: "Whoops! Something went wrong"}))).
		ServeHTTP(res, req)
}
