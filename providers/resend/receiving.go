// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package resend

import (
	"fmt"
	"net/mail"
	"time"

	"github.com/resend/resend-go/v3"

	"github.com/immanent-tech/foragd/validation"
)

type ReceivedEmail struct {
	*resend.ReceivedEmail
}

func (e *ReceivedEmail) GetID() string {
	return e.Id
}

func (e *ReceivedEmail) Timestamp() time.Time {
	ts, err := time.Parse("2006-01-02T15:04:05.999999999+07:00", e.CreatedAt)
	if err != nil {
		return time.Now().UTC()
	}
	return ts.UTC()
}

func (e *ReceivedEmail) GetSubject() string {
	return validation.SanitizeString(e.Subject)
}

func (e *ReceivedEmail) GetBody() string {
	switch {
	case e.Html != "":
		return validation.SanitizeString(e.Html)
	case e.Text != "":
		return validation.SanitizeString(e.Text)
	default:
		return ""
	}
}

func (e *ReceivedEmail) GetFrom() *mail.Address {
	from, err := mail.ParseAddress(e.From)
	if err != nil {
		return &mail.Address{
			Address: e.From,
		}
	}
	return from
}

// Valid returns a non-nil error when the ReceivedEmail contains invalid fields.
func (e *ReceivedEmail) Valid() error {
	if err := validation.Validate.Var(e.From, "required,email"); err != nil {
		return fmt.Errorf("%w: from: %w", ErrInvalidEmail, err)
	}
	if e.GetSubject() == "" {
		return fmt.Errorf("%w: empty subject")
	}
	if e.GetBody() == "" {
		return fmt.Errorf("%w: empty body")
	}
	return nil
}
