// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"errors"
	"fmt"
	"strings"
)

var ErrUnknown = errors.New("an unknown error occurred")

// NewSuccessMessage creates a new UserMessage indicating success with the given summary and (optional) details.
func NewSuccessMessage(summary string, details string) *UserMessage {
	return &UserMessage{
		Status:  UserMessageStatusSuccess,
		Summary: summary,
		Details: details,
	}
}

// NewErrorMessage creates a new UserMessage indicating an error with the given summary and (optional) details.
func NewErrorMessage(summary string, details string) *UserMessage {
	return &UserMessage{
		Status:  UserMessageStatusError,
		Summary: summary,
		Details: details,
	}
}

// NewWarningMessage creates a new UserMessage indicating a warning with the given summary and (optional) details.
func NewWarningMessage(summary string, details string) *UserMessage {
	return &UserMessage{
		Status:  UserMessageStatusWarning,
		Summary: summary,
		Details: details,
	}
}

// NewInfoMessage creates a new UserMessage indicating informational details with the given summary and (optional) details.
func NewInfoMessage(summary string, details string) *UserMessage {
	return &UserMessage{
		Status:  UserMessageStatusInfo,
		Summary: summary,
		Details: details,
	}
}

// HasDetails returns a boolean indicating whether the message has additional details.
func (msg *UserMessage) HasDetails() bool {
	return msg.Details != ""
}

// String returns the message as a formatted string. This allows Message to satisfy the Stringer interface.
func (msg *UserMessage) String() string {
	var str strings.Builder
	str.WriteString(fmt.Sprintf("%s: %s", strings.ToTitle(string(msg.Status)), msg.Summary))
	if msg.Details != "" {
		str.WriteString("\n" + msg.Details)
	}
	return str.String()
}

// IsSuccess returns true when the message indicates success.
func (msg *UserMessage) IsSuccess() bool {
	return msg.Status == UserMessageStatusSuccess
}

// IsError returns true when the message indicates an error.
func (msg *UserMessage) IsError() bool {
	return msg.Status == UserMessageStatusError
}

// IsWarning returns true when the message indicates a warning.
func (msg *UserMessage) IsWarning() bool {
	return msg.Status == UserMessageStatusWarning
}

// IsInfo returns true when the message indicates informational status.
func (msg *UserMessage) IsInfo() bool {
	return msg.Status == UserMessageStatusInfo
}
