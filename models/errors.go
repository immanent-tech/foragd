// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"

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
	ErrForbidden = &APIError{
		InternalError: errors.New("forbidden"),
		StatusCode:    http.StatusForbidden,
		UserMessage: NewWarningMessage(
			"Access restricted.",
			"Your access has been forbidden. Please contact support if you believe this is an error.",
		),
	}
	// ErrSubscriptionLimitExceeded indicates that the user has exceeded their subscription limit.
	ErrSubscriptionLimitExceeded = &APIError{
		InternalError: errors.New("subscription limit exceeded"),
		StatusCode:    http.StatusTooManyRequests,
		UserMessage: NewWarningMessage(
			"Exceeded subscription limit.",
			"You have exceeded your subscription limit ("+strconv.Itoa(
				MaxSubscriptions,
			)+"). Please remove some subscriptions to get under this limit before continuing.",
		),
	}
	// ErrEmailNewsletterLimitExceeded indicates that the user has exceeded their email newsletter limit.
	ErrEmailNewsletterLimitExceeded = &APIError{
		InternalError: errors.New("email newsletter limit exceeded"),
		StatusCode:    http.StatusTooManyRequests,
		UserMessage: NewWarningMessage(
			"Exceeded email newsletter limit.",
			"You have exceeded your email newsletter limit ("+strconv.Itoa(
				MaxEmailNewsletters,
			)+"). Please remove some email newsletter subscriptions to get under this limit before continuing.",
		),
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
	if apiErr, ok := errors.AsType[interface {
		HTTPStatus() int
		error
	}](err); ok {
		return apiErr.HTTPStatus()
	}
	return http.StatusInternalServerError
}

type ErrorOption func(*APIError)

func WithUserErrorSummary(msg string) ErrorOption {
	return func(a *APIError) {
		if a.UserMessage == nil {
			a.UserMessage = &UserMessage{}
		}
		a.UserMessage.Summary = msg
	}
}

func WithUserErrorDescription(msg string) ErrorOption {
	return func(a *APIError) {
		if a.UserMessage == nil {
			a.UserMessage = &UserMessage{}
		}
		a.UserMessage.Details = &msg
	}
}

func NewAPIError(status int, err error, options ...ErrorOption) *APIError {
	apiErr := &APIError{
		StatusCode:    status,
		InternalError: err,
	}
	for option := range slices.Values(options) {
		option(apiErr)
	}
	if apiErr.UserMessage == nil {
		apiErr.UserMessage = NewErrorMessage(
			"Server could not complete request",
			"This might be temporary, please try again.",
		)
	}
	return apiErr
}
