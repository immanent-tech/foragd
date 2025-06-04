// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"fmt"
	"net/http"
)

// IsError returns a boolean indicating whether the response is an error. The criteria for being an error is having a
// non-nil InternalError field.
func (r *Response) IsError() bool {
	if r == nil {
		return false
	}
	return r.InternalError != nil
}

func (r *Response) String() string {
	switch {
	case r.UserMessage != nil:
		return r.UserMessage.String()
	case r.IsError():
		return r.InternalError.Error()
	default:
		return fmt.Sprintf("%d: An unknown error occurred.", r.StatusCode)
	}
}

func RespTemporaryIssue(msg string, err error) *Response {
	return &Response{
		StatusCode:    http.StatusNoContent,
		InternalError: err,
		UserMessage: &UserMessage{
			Status:  UserMessageStatusWarning,
			Summary: msg,
		},
	}
}

func RespSuccess(msg string) *Response {
	return &Response{
		StatusCode: http.StatusOK,
		UserMessage: &UserMessage{
			Status:  UserMessageStatusSuccess,
			Summary: msg,
		},
	}
}

func RespNonCriticalError(msg string, err error) *Response {
	return &Response{
		StatusCode:    http.StatusInternalServerError,
		InternalError: err,
		UserMessage: &UserMessage{
			Status:  UserMessageStatusWarning,
			Summary: msg,
		},
	}
}

func RespServerError(msg string, err error) *Response {
	return &Response{
		StatusCode:    http.StatusInternalServerError,
		InternalError: err,
		UserMessage: &UserMessage{
			Status:  UserMessageStatusError,
			Summary: msg,
		},
	}
}

func RespForbidden(msg string, err error) *Response {
	return &Response{
		StatusCode:    http.StatusForbidden,
		InternalError: err,
		UserMessage: &UserMessage{
			Status:  UserMessageStatusError,
			Summary: msg,
		},
	}
}

func RespInvalidUser() *Response {
	return &Response{
		StatusCode:    http.StatusInternalServerError,
		InternalError: ErrNoUserCtx,
		UserMessage: &UserMessage{
			Status:  UserMessageStatusError,
			Summary: "Invalid or expired session.",
		},
	}
}
