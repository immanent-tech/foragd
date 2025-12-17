// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"net/http"

	slogctx "github.com/veqryn/slog-context"
)

// ErrInvalidMimeType indicates that the mime type is not valid.
var ErrInvalidMimeType = errors.New("invalid mime type")

var ErrUserNotFound = NewAPIError(
	errors.New("no valid user found"), //nolint:err113 // this is wrapped.
	http.StatusForbidden,
)

// NewAPIError creates a new API error with the given internal error  and status code.
func NewAPIError(err error, status int) error {
	return &APIError{
		InternalError: err,
		StatusCode:    status,
	}
}

func (e *APIError) Error() string { return e.InternalError.Error() }
func (e *APIError) Unwrap() error { return e.InternalError }

// HTTPStatus returns the status code of the API error.
func (e *APIError) HTTPStatus() int { return e.StatusCode }

// WriteLog writes the APIError to the log at the appropriate level.
func (e *APIError) WriteLog(ctx context.Context) {
	switch {
	case e.HTTPStatus() < 400: //nolint:mnd // easier to read as a number.
		slogctx.FromCtx(ctx).DebugContext(ctx, e.Error())
	case e.HTTPStatus() < 500: //nolint:mnd // easier to read as a number.
		slogctx.FromCtx(ctx).WarnContext(ctx, e.Error())
	default:
		slogctx.FromCtx(ctx).ErrorContext(ctx, e.Error())
	}
}

// HTTPStatus returns the HTTP status included in err. If err is nil, this
// function returns 0. If err is non-nil, and does not include an HTTP status,
// a default value of [net/http.StatusInternalServerError] is returned.
func HTTPStatus(err error) int {
	if err == nil {
		return 0
	}
	var apiErr interface {
		error
		HTTPStatus() int
	}
	if errors.As(err, &apiErr) {
		return apiErr.HTTPStatus()
	}
	return http.StatusInternalServerError
}
