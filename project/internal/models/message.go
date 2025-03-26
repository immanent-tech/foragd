// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"fmt"
	"slices"
	"strings"
)

// MessageOption is a functional option to apply to a Message.
type MessageOption func(*Message)

func (msg *Message) HasDetails() bool {
	return msg.Details != nil
}

// String returns the message as a formatted string. This allows Message to satisfy the Stringer interface.
func (msg *Message) String() string {
	var str strings.Builder
	str.WriteString(fmt.Sprintf("%s: %s", strings.ToTitle(string(msg.Status)), msg.Summary))
	if msg.Details != nil {
		str.WriteString(fmt.Sprintf(" (%s)", *msg.Details))
	}
	return str.String()
}

// Error returns an error string representing the Message. This allows Message to satisfy the Error interface and be
// used as an error.
func (msg *Message) Error() string {
	return fmt.Sprintf("%s: %v", msg.Summary, msg.Err)
}

// WithStatus option sets the level on the notice. If this option is not
// provided, the notice will default to the "info" level.
func WithStatus(status MessageStatus) MessageOption {
	return func(notice *Message) {
		notice.Status = status
	}
}

// WithDetails option sets the details, or extra information on the Message.
func WithDetails(details string) MessageOption {
	return func(notice *Message) {
		notice.Details = &details
	}
}

// WithError option sets the internal error for the Message.
func WithError(err error) MessageOption {
	return func(notice *Message) {
		notice.Err = err
	}
}

func NewMessage(summary string, options ...MessageOption) *Message {
	notice := &Message{
		Summary: summary,
		Status:  MessageStatusInfo,
	}

	for option := range slices.Values(options) {
		option(notice)
	}

	return notice
}
