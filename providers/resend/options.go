// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package resend

import "net/mail"

type HasSubject interface {
	*Template | *ReceivedEmail

	SetSubject(subject string)
}

// WithSubject option sets the subject field for the email.
func WithSubject[T HasSubject](subject string) func(T) {
	return func(t T) {
		t.SetSubject(subject)
	}
}

type HasReplyTo interface {
	*Template | *ReceivedEmail

	SetReplyTo(replyTo any)
}

// WithReplyTo option sets the reply-to field for the email.
func WithReplyTo[T HasReplyTo](replyTo any) func(T) {
	return func(t T) {
		t.SetReplyTo(replyTo)
	}
}

type HasFrom interface {
	*Template | *ReceivedEmail

	SetFrom(replyTo any)
}

// WithFrom option sets the from field for the email.
func WithFrom[T HasFrom](from any) func(T) {
	return func(t T) {
		t.SetFrom(from)
	}
}

type Address interface {
	string | *mail.Address
}
