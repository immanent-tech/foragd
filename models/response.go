// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"

	"github.com/a-h/templ"
)

var (
	ErrHTMXRequired = errors.New("htmx is required")
	ErrInvalidInput = errors.New("invalid or unknown input")
)

// ResponseOption is a functional option to apply to a response.
type ResponseOption func(*Response)

// WithResponseStatusCode option applies the given HTTP status code to the response. If this option is not provided, a
// default 200 status code is used.
func WithResponseStatusCode(status int) ResponseOption {
	return func(r *Response) {
		r.StatusCode = status
	}
}

// WithResponseTemplate assigns the given template to the response. If this option is not provided, an empty body will
// be written when the response is rendered.
func WithResponseTemplate(template templ.Component) ResponseOption {
	return func(r *Response) {
		r.Template = template
	}
}

// WithResponseError assigns an error message that will be written as a log message when the response is rendered.
func WithResponseError(err error) ResponseOption {
	return func(r *Response) {
		r.InternalError = err
	}
}

// NewResponse creates a new response with the given options.
func NewResponse(options ...ResponseOption) *Response {
	resp := &Response{}

	for option := range slices.Values(options) {
		option(resp)
	}

	// Set a default 200: OK status code if not set by option.
	if resp.StatusCode == 0 {
		resp.StatusCode = http.StatusOK
	}

	return resp
}

func (r *Response) IsNotFound() bool {
	if r != nil {
		return r.StatusCode == http.StatusNotFound
	}
	return false
}

func (r *Response) String() string {
	if r == nil {
		return "unknown error"
	}
	switch {
	case r.InternalError != nil:
		return fmt.Sprintf("%d: %s", r.StatusCode, r.InternalError.Error())
	default:
		return http.StatusText(r.StatusCode)
	}
}

func (r *Response) Error() string {
	return r.String()
}

// Render will render the template in the response. This satisfies the templ.Component interface.
func (r *Response) Render(ctx context.Context, w io.Writer) error {
	if r.Template != nil {
		return r.Template.Render(ctx, w)
	}
	return nil
}

func RespErrUnauthorized() *Response {
	return &Response{
		StatusCode:    http.StatusUnauthorized,
		InternalError: ErrInvalidUser,
	}
}

func RespErrBackend(err error) *Response {
	return &Response{
		StatusCode:    http.StatusInternalServerError,
		InternalError: err,
	}
}

func RespForbidden(err error) *Response {
	return &Response{
		StatusCode:    http.StatusForbidden,
		InternalError: err,
	}
}

func RespInvalidInput() *Response {
	return &Response{
		StatusCode:    http.StatusNoContent,
		InternalError: ErrInvalidInput,
	}
}

func RespNotFound(err error) *Response {
	return &Response{
		StatusCode:    http.StatusNotFound,
		InternalError: err,
	}
}

// RespInternalServerError will create a new response indicating a 503 server error with the given template for display
// to the user.
func RespInternalServerError(err error, template templ.Component) *Response {
	return NewResponse(
		WithResponseStatusCode(http.StatusInternalServerError),
		WithResponseError(err),
		WithResponseTemplate(template),
	)
}
