// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"slices"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/pkg/htmx"

	"github.com/immanent-tech/foragd/models"
	gerror "github.com/immanent-tech/foragd/providers/google/error"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/partials"
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
	*Page

	err     *models.APIError
	referer string
}

// HandleInternalError handles display errors on internal pages (pages accessible to logged in users). If the passed in
// error can be unwrapped as a models.APIError, the status code and value of the APIError is used and the passed in
// status code is ignored. Otherwise a new APIError will be generated from the status code and error.
func (m *Manager) HandleInternalError(status int, err error, options ...ErrorOption) http.HandlerFunc {
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
		page := &InternalError{
			Page: m.NewPage(req.URL.Path,
				WithSummary(opts.UserMessage.Summary),
				WithDescription(*opts.UserMessage.Details),
			),
		}

		apiErr, ok := errors.AsType[*models.APIError](err)
		if ok {
			// Write appropriately leveled log message.
			apiErr.WriteLog(req.Context())
			// For 500+ errors, log to GCP error console.
			if apiErr.HTTPStatus() >= 500 {
				gerror.ReportError(apiErr)
			}
			// Write response.
			if apiErr.UserMessage == nil {
				// Add a generic user message if one isn't already set.
				apiErr.UserMessage = opts.UserMessage
			}
		} else {
			if status == 0 {
				status = http.StatusInternalServerError
			}
			if status >= 500 {
				gerror.ReportError(apiErr)
			}
			apiErr = models.NewAPIError(status, err, models.WithUserMessage(opts.UserMessage))
			apiErr.WriteLog(req.Context())
		}
		page.err = apiErr
		page.referer = req.URL.Path
		res.WriteHeader(apiErr.HTTPStatus())
		// Don't cache errors.
		res.Header().Set("Cache-Control", "no-store")
		if req.Method == http.MethodGet {
			res.Header().Set(htmx.HeaderRetarget, templates.ContentID.Target())
			page.template = templates.InternalError(
				models.UserFromCtx(req.Context()),
				page.referer,
				page.err.UserMessage,
			)
		} else {
			res.Header().Set(htmx.HeaderReswap, "none")
			page.template = partials.Notification(page.err.UserMessage)
		}

		page.Render().ServeHTTP(res, req)
	}
}

// ExternalError represents errors shown on external (public) pages.
type ExternalError struct {
	*Page
}

// HandleExternalError handles display errors on external pages.
func (m *Manager) HandleExternalError(err error) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		page := &ExternalError{
			Page: m.NewPage(req.URL.Path,
				WithSummary("Whoops! Something went wrong"),
				WithDescription("We could not display the page you requested"),
			),
		}
		// Don't cache errors.
		res.Header().Set("Cache-Control", "no-store")

		if apiErr, ok := errors.AsType[*models.APIError](err); ok {
			// Write appropriately leveled log message.
			apiErr.WriteLog(req.Context())
			// For 500+ errors, log to GCP error console.
			if apiErr.HTTPStatus() >= 500 {
				gerror.ReportError(apiErr)
			}
			// Write response.
			page.template = templates.ExternalError(models.UserFromCtx(req.Context()), apiErr.GetUserMessage())
			res.WriteHeader(apiErr.HTTPStatus())
			page.Render().ServeHTTP(res, req)
		} else {
			slogctx.FromCtx(req.Context()).Error("Unknown error occurred.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}
