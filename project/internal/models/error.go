// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import "fmt"

// Error satisfies the Error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s (%v)", e.Context, e.Message, e.Err)
}

// WrapError will wrap a low-level error with the given context and
// user-friendly message as a new Error object.
func WrapError(err error, context, message string) *Error {
	return &Error{
		Context: context,
		Message: message,
		Err:     err,
	}
}
