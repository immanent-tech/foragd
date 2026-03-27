// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package resend

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"sync"

	"github.com/cenkalti/backoff/v5"
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
		encoded, err := EncodeEmail(e.To[0])
		if err != nil {
			return nil, fmt.Errorf("encode email: %w", err)
		}
		req.Headers = map[string]string{
			"List-Unsubscribe-Post": "List-Unsubscribe=One-Click",
			"List-Unsubscribe":      "https://foragd.app/unsubscribe/" + encoded,
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
func BatchSendEmails(ctx context.Context, emails ...*Email) (BatchSendResponse, error) {
	client, err := loadClient()
	if err != nil {
		return nil, fmt.Errorf("load client: %w", err)
	}

	const maxBatchSize = 100
	resp := make(BatchSendResponse, 0, len(emails))

	// Send emails in batches up to maxBatchSize.
	for idx := 0; idx < len(emails); idx += maxBatchSize {
		// Generate batch.
		j := min(idx+maxBatchSize, len(emails))
		batch := make([]*resend.SendEmailRequest, 0)
		for email := range slices.Values(emails[idx:j]) {
			req, err := email.createRequest()
			if err != nil {
				resp = append(resp, SendResponse{
					To:    email.To,
					Error: err,
				})
				continue
			}
			batch = append(batch, req)
		}

		batchResp, err := backoff.Retry(
			context.TODO(),
			batchOperation(ctx, client, batch),
			backoff.WithBackOff(backoff.NewExponentialBackOff()),
		)
		if err != nil {
			// On batch send error, record all emails in batch as failed with same error.
			for email := range slices.Values(batch) {
				resp = append(resp, SendResponse{
					To:    email.To,
					Error: err,
				})
			}
			continue
		}

		// Record any failures results.
		for idx, email := range batch {
			if slices.ContainsFunc(batchResp.Errors, func(e resend.BatchError) bool {
				return e.Index == idx
			}) {
				resp = append(resp, SendResponse{
					To:    email.To,
					Error: errors.New(batchResp.Errors[idx].Message),
				})
			} else {
				resp = append(resp, SendResponse{To: email.To})
			}
		}
	}

	slogctx.FromCtx(ctx).Debug("Batch sent emails",
		slog.Int("count", len(resp)),
	)

	return resp, nil
}

func batchOperation(
	ctx context.Context,
	client *resend.Client,
	batch []*resend.SendEmailRequest,
) func() (*resend.BatchEmailResponse, error) {
	return func() (*resend.BatchEmailResponse, error) {
		// Send batch.
		resp, err := client.Batch.SendWithOptions(
			ctx,
			batch,
			&resend.BatchSendEmailOptions{BatchValidation: resend.BatchValidationPermissive},
		)

		if rateLimitErr, ok := errors.AsType[*resend.RateLimitError](
			err,
		); ok &&
			rateLimitErr.Message == "rate_limit_exceeded" {
			if seconds, err := strconv.ParseInt(rateLimitErr.RetryAfter, 10, 64); err == nil {
				return nil, backoff.RetryAfter(int(seconds))
			}
		} else {
			return resp, fmt.Errorf("batch failed: %w", err)
		}
		return resp, nil
	}
}

// SendResponse contains details about an individual email sent status.
type SendResponse struct {
	To    []string
	Error error
}

// BatchSendResponse contains data related to a batch email request.
type BatchSendResponse []SendResponse
