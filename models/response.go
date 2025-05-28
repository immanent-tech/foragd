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
