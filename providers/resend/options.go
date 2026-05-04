// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package resend

import "net/mail"

type HasSubject interface {
	*Template | *ReceivedEmail | *Email

	SetSubject(subject string)
}

// WithSubject option sets the subject field for the email.
func WithSubject[T HasSubject](subject string) func(T) {
	return func(t T) {
		t.SetSubject(subject)
	}
}

type HasReplyTo interface {
	*Template | *ReceivedEmail | *Email

	SetReplyTo(replyTo any)
}

// WithReplyTo option sets the reply-to field for the email.
func WithReplyTo[T HasReplyTo](replyTo any) func(T) {
	return func(t T) {
		t.SetReplyTo(replyTo)
	}
}

type HasFrom interface {
	*Template | *ReceivedEmail | *Email

	SetFrom(replyTo any)
}

// WithFrom option sets the from field for the email.
func WithFrom[T HasFrom](from any) func(T) {
	return func(t T) {
		t.SetFrom(from)
	}
}

type HasTo interface {
	*Email

	SetTo(to any)
}

// WithTo option sets the to field for the email. Can be specified multiple times when sending an email to send to
// multiple recipients.
func WithTo[T HasTo](to any) func(T) {
	return func(t T) {
		t.SetTo(to)
	}
}

type HasAttachment interface {
	*Email

	SetRemoteAttachment(attachment *Attachment)
}

// WithRemoteAttachment option defines a remote file attachment to add to the email.
func WithRemoteAttachment[T HasAttachment](attachment *Attachment) func(T) {
	return func(t T) {
		if attachment.Path != "" && attachment.Filename != "" {
			t.SetRemoteAttachment(attachment)
		}
	}
}

type Address interface {
	string | *mail.Address
}
