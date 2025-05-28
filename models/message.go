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
	return msg.Details != nil
}

// String returns the message as a formatted string. This allows Message to satisfy the Stringer interface.
func (msg *UserMessage) String() string {
	var str strings.Builder
	str.WriteString(fmt.Sprintf("%s: %s", strings.ToTitle(string(msg.Status)), msg.Summary))
	if msg.Details != nil {
		str.WriteString("\n" + *msg.Details)
	}
	return str.String()
}

// CSVString writes the message out in a format suitable as a record in a CSV file.
func (msg *UserMessage) CSVString() string {
	if msg == nil {
		return ""
	}
	csv := fmt.Sprintf(`%s,%q`, msg.Status, msg.Summary)
	if msg.Details != nil {
		csv += fmt.Sprintf(`,%q`, *msg.Details)
	} else {
		csv += `,""`
	}
	return csv + "\n"
}

// Error returns an error string representing the Message. This allows Message to satisfy the Error interface and be
// used as an error.
func (msg *UserMessage) Error() string {
	return msg.String()
}
