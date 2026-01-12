// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package resend

type WebhookEmailReceieved struct {
	Type      string        `json:"type,omitempty"`
	CreatedAt string        `json:"created_at,omitempty"`
	Data      EmailRecieved `json:"data,omitempty"`
}

type Attachment struct {
	Id                 string `json:"id,omitempty"`
	Filename           string `json:"filename,omitempty"`
	ContentType        string `json:"content_type,omitempty"`
	ContentDisposition string `json:"content_disposition,omitempty"`
	ContentId          string `json:"content_id,omitempty"`
}

type EmailRecieved struct {
	EmailId     string       `json:"email_id,omitempty"`
	CreatedAt   string       `json:"created_at,omitempty"`
	From        string       `json:"from,omitempty"`
	To          []string     `json:"to,omitempty"`
	Bcc         []string     `json:"bcc,omitempty"`
	Cc          []string     `json:"cc,omitempty"`
	MessageId   string       `json:"message_id,omitempty"`
	Subject     string       `json:"subject,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
}
