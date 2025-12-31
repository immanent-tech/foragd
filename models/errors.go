// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	slogctx "github.com/veqryn/slog-context"
)

var (
	// ErrNotFound indicates the backend API returned no results.
	ErrNotFound = &APIError{
		InternalError: errors.New("not found"),
		StatusCode:    http.StatusNotFound,
	}
	// ErrInvalidAPIResult indicates that the backend API returned unexpected, invalid or an otherwise incorrect response.
	ErrInvalidAPIResult = &APIError{
		InternalError: errors.New("invalid backend API result"),
		StatusCode:    http.StatusInternalServerError,
	}
	// ErrInvalidParams indicates that invalid parameters were received or generated.
	ErrInvalidParams = &APIError{
		InternalError: errors.New("invalid parameters"),
		StatusCode:    http.StatusUnprocessableEntity,
	}
)

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

// GetUserMessage returns a UserMessage associated with the error. It will return an appropriate generic error message
// or warning where no message is already defined.
func (e *APIError) GetUserMessage() *UserMessage {
	if e.UserMessage != nil {
		return e.UserMessage
	}
	switch {
	case e.InternalError != nil:
		return NewErrorMessage(
			"Server could not complete request",
			"This might be temporary, please try again.",
		)
	default:
		return NewWarningMessage(
			"There was a problem with the request.",
			"Please check any inputs and resubmit the request.",
		)
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

// es2APIError converts an Elastic package error into a APIError.
func es2APIError(msg string, err error) error {
	var esErr *types.ElasticsearchError
	if errors.As(err, &esErr) {
		return &APIError{
			InternalError: fmt.Errorf("%s: %s: %s", msg, esErr.ErrorCause.Type, *esErr.ErrorCause.Reason),
			StatusCode:    esErr.Status,
		}
	}
	return &APIError{
		InternalError: fmt.Errorf("%s: %w", msg, err),
		StatusCode:    http.StatusInternalServerError,
	}
}
