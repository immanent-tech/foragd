// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package resend

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"

	"github.com/resend/resend-go/v3"
	slogctx "github.com/veqryn/slog-context"
)

type email struct {
	*resend.Email

	template *resend.EmailTemplate
	tags     []resend.Tag

	mu sync.Mutex
}

// EmailOption is a functional option to apply to an email.
type EmailOption func(*email)

// To option sets the to address of an email.
func To(to ...string) EmailOption {
	return func(e *email) {
		e.To = append(e.To, to...)
	}
}

// Cc option sets the cc address of an email.
func Cc(cc ...string) EmailOption {
	return func(e *email) {
		e.Cc = append(e.Cc, cc...)
	}
}

// Bcc option sets the bcc address of an email.
func Bcc(bcc ...string) EmailOption {
	return func(e *email) {
		e.Bcc = append(e.Bcc, bcc...)
	}
}

// WithVariable option assigns a value to the given template variable in the email template.
func WithVariable(key string, value any) EmailOption {
	return func(e *email) {
		e.mu.Lock()
		defer e.mu.Unlock()
		e.template.Variables[key] = value
	}
}

// WithTag option applies the given tag to the email.
func WithTag(key string, value string) EmailOption {
	return func(e *email) {
		e.mu.Lock()
		defer e.mu.Unlock()
		e.tags = append(e.tags, resend.Tag{Name: key, Value: value})
	}
}

// SendTemplatedEmail sends the template with the given id to the given address, with any additional template options
// applied.
func SendTemplatedEmail(ctx context.Context, templateID string, options ...EmailOption) error {
	template := &email{
		Email: &resend.Email{},
		template: &resend.EmailTemplate{
			Id:        templateID,
			Variables: make(map[string]interface{}),
		},
	}

	for option := range slices.Values(options) {
		option(template)
	}

	// Check validity.
	switch {
	case len(template.To) == 0 || len(template.Bcc) == 0 || len(template.Cc) == 0:
		// Must have a To, Cc, or Bcc.
		return fmt.Errorf("%w: no address(es) specified", ErrInvalidEmail)
	case template.template.Id == "":
		// Must specify a template ID.
		return fmt.Errorf("%w: no template specified", ErrInvalidEmail)
	}

	client, err := loadClient()
	if err != nil {
		return fmt.Errorf("load client: %w", err)
	}

	params := &resend.SendEmailRequest{
		To:       template.To,
		Cc:       template.Cc,
		Bcc:      template.Bcc,
		Template: template.template,
		Tags:     template.tags,
	}

	_, err = client.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("send template email: %w", err)
	}

	slogctx.FromCtx(ctx).Debug("Templated email sent.",
		slog.String("template", templateID),
	)

	return nil
}
