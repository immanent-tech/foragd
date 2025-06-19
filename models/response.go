// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"errors"
	"fmt"
	"net/http"
)

var (
	ErrInvalidUser  = errors.New("user is invalid")
	ErrHTMXRequired = errors.New("htmx is required")
	ErrInvalidInput = errors.New("invalid or unknown input")
)

func NewResponse(status int, err error) *Response {
	resp := &Response{
		StatusCode: status,
	}
	if err != nil {
		resp.InternalError = err
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
