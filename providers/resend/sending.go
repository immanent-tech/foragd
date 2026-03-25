// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package resend

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"

	"github.com/resend/resend-go/v3"
	slogctx "github.com/veqryn/slog-context"
)

// Email is an email to be processed with the resend API.
type Email struct {
	*resend.Email

	template *resend.EmailTemplate
	tags     []resend.Tag

	mu sync.Mutex
}

// NewTemplatedEmail creates a new email using the given template and with any options specified.
func NewTemplatedEmail(templateID string, options ...EmailOption) (*Email, error) {
	email := &Email{
		Email: &resend.Email{},
		template: &resend.EmailTemplate{
			Id:        templateID,
			Variables: make(map[string]interface{}),
		},
	}

	for option := range slices.Values(options) {
		option(email)
	}

	if err := email.Valid(); err != nil {
		return nil, fmt.Errorf("validate email: %w", err)
	}

	return email, nil
}

func (e *Email) Valid() error {
	if len(e.To) == 0 && len(e.Bcc) == 0 && len(e.Cc) == 0 {
		// Must have a To, Cc, or Bcc.
		return fmt.Errorf("%w: no address(es) specified", ErrInvalidEmail)
	}
	if e.template.Id == "" {
		// Must specify a template ID.
		return fmt.Errorf("%w: no template specified", ErrInvalidEmail)
	}

	return nil
}

func (e *Email) createRequest() (*resend.SendEmailRequest, error) {
	req := &resend.SendEmailRequest{
		To:       e.To,
		Cc:       e.Cc,
		Bcc:      e.Bcc,
		Template: e.template,
		Tags:     e.tags,
	}

	// If the email has a category tag with the value "promotional" and appropriate unsubscribe headers.
	if slices.ContainsFunc(e.tags, func(t resend.Tag) bool {
		return t.Name == "category" && t.Value == "promotional"
	}) {
		encoded, err := EncodeEmail(e.To[0], cfg.Key, cfg.Salt)
		if err != nil {
			return nil, fmt.Errorf("encode email: %w", err)
		}
		req.Headers = map[string]string{
			"List-Unsubscribe-Post": "List-Unsubscribe=One-Click",
			"List-Unsubscribe":      "https://foragd.app/email/unsubscribe/" + encoded,
		}
	}

	return req, nil
}

// EmailOption is a functional option to apply to an email.
type EmailOption func(*Email)

// To option sets the to address of an email.
func To(to ...string) EmailOption {
	return func(e *Email) {
		e.To = append(e.To, to...)
	}
}

// Cc option sets the cc address of an email.
func Cc(cc ...string) EmailOption {
	return func(e *Email) {
		e.Cc = append(e.Cc, cc...)
	}
}

// Bcc option sets the bcc address of an email.
func Bcc(bcc ...string) EmailOption {
	return func(e *Email) {
		e.Bcc = append(e.Bcc, bcc...)
	}
}

// WithVariable option assigns a value to the given template variable in the email template.
func WithVariable(key string, value any) EmailOption {
	return func(e *Email) {
		e.mu.Lock()
		defer e.mu.Unlock()
		e.template.Variables[key] = value
	}
}

// WithTag option applies the given tag to the email.
func WithTag(key string, value string) EmailOption {
	return func(e *Email) {
		e.mu.Lock()
		defer e.mu.Unlock()
		e.tags = append(e.tags, resend.Tag{Name: key, Value: value})
	}
}

// SendEmail sends the given email.
func SendEmail(ctx context.Context, email *Email) error {
	client, err := loadClient()
	if err != nil {
		return fmt.Errorf("load client: %w", err)
	}

	req, err := email.createRequest()
	if err != nil {
		return fmt.Errorf("create email request: %w", err)
	}

	_, err = client.Emails.SendWithContext(ctx, req)
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	slogctx.FromCtx(ctx).Debug("Email sent.",
		slog.String("template", email.template.Id),
	)

	return nil
}

// BatchSendEmails sends the given emails in a batch request.
func BatchSendEmails(ctx context.Context, emails ...*Email) (*BatchEmailResponse, error) {
	client, err := loadClient()
	if err != nil {
		return nil, fmt.Errorf("load client: %w", err)
	}

	// Create email batch.
	batch := make([]*resend.SendEmailRequest, 0, len(emails))
	for email := range slices.Values(emails) {
		req, err := email.createRequest()
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Could not create email request",
				slog.Any("error", err),
			)
		}

		batch = append(batch, req)
	}

	// Send batch.
	resp, err := client.Batch.SendWithContext(ctx, batch)
	if err != nil {
		return nil, fmt.Errorf("batch send emails: %w", err)
	}

	slogctx.FromCtx(ctx).Debug("Batch sent emails",
		slog.Int("count", len(resp.Data)),
	)

	return &BatchEmailResponse{batch: batch, BatchEmailResponse: resp}, nil
}

// BatchEmailResponse contains data related to a batch email request.
type BatchEmailResponse struct {
	*resend.BatchEmailResponse

	batch []*resend.SendEmailRequest
}

// GetFailed returns a map of email addresses that were not sent an email in the batch and reason.
func (r *BatchEmailResponse) GetFailed() map[string]error {
	errs := make(map[string]error)
	for batchErr := range slices.Values(r.Errors) {
		email := r.batch[batchErr.Index]
		for to := range slices.Values(email.To) {
			errs[to] = errors.New(batchErr.Message)
		}
	}
	return errs
}
