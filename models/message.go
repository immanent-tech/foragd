// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"errors"
	"fmt"
	"strings"
)

var ErrUnknown = errors.New("an unknown error occurred")

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

func SuccessUserMessage(summary string, details string) *UserMessage {
	return &UserMessage{
		Status:  UserMessageStatusSuccess,
		Summary: summary,
		Details: details,
	}
}

func FailedUserMessage(summary string, details string) *UserMessage {
	return &UserMessage{
		Status:  UserMessageStatusError,
		Summary: summary,
		Details: details,
	}
}
