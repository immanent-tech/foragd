// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import "errors"

var (
	// ErrNoUserCtx indicates the user object was not found in the context.
	ErrNoUserCtx = errors.New("no valid user in context")
	// ErrInvalidMimeType indicates that the mime type is not valid.
	ErrInvalidMimeType = errors.New("invalid mime type")
)
