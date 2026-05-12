// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package resend

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
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

	template    *resend.EmailTemplate
	tags        []resend.Tag
	attachments []*resend.Attachment

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

func (e *Email) SetSubject(subject string) {
	e.Subject = subject
}

func (e *Email) SetReplyTo(replyTo any) {
	switch v := replyTo.(type) {
	case string:
		e.ReplyTo = append(e.ReplyTo, v)
	case *mail.Address:
		e.ReplyTo = append(e.ReplyTo, v.String())
	}
}

func (e *Email) SetFrom(from any) {
	switch v := from.(type) {
	case string:
		e.From = v
	case *mail.Address:
		e.From = v.String()
	}
}

func (e *Email) SetTo(to any) {
	switch v := to.(type) {
	case string:
		e.To = append(e.To, v)
	case *mail.Address:
		e.To = append(e.To, v.String())
	}
}

func (e *Email) SetRemoteAttachment(attachment *Attachment) {
	e.attachments = append(e.attachments, attachment.Attachment)
}

func (e *Email) Valid() error {
	if len(e.To) == 0 && len(e.Bcc) == 0 && len(e.Cc) == 0 {
		// Must have a To, Cc, or Bcc.
		return fmt.Errorf("%w: no address(es) specified", ErrInvalidEmail)
	}
	// if e.template.Id == "" {
	// 	// Must specify a template ID.
	// 	return fmt.Errorf("%w: no template specified", ErrInvalidEmail)
	// }

	return nil
}

func (e *Email) createRequest() (*resend.SendEmailRequest, error) {
	req := &resend.SendEmailRequest{
		To:   e.To,
		Cc:   e.Cc,
		Bcc:  e.Bcc,
		Tags: e.tags,
	}
	if e.template != nil && e.template.Id != "" {
		// If there is a template, use that.
		req.Template = e.template
	} else {
		// Manually fill out required fields.
		req.From = e.From
		if len(e.ReplyTo) > 0 {
			req.ReplyTo = e.ReplyTo[0]
		}
		req.Subject = e.Subject
		req.Text = e.Text
		req.Html = e.Html
	}
	if len(e.attachments) > 0 {
		req.Attachments = e.attachments
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

// WithTextContent option sets the text content of the email (shown in clients that don't support HTML emails).
func WithTextContent(text string) EmailOption {
	return func(email *Email) {
		email.Text = text
	}
}

// WithHTMLContent option sets the html content of the email. When using this option, it is advised to also use the
// WithTextContent option to set the text content of the email shown to clients that don't support html emails.
func WithHTMLContent(html string) EmailOption {
	return func(email *Email) {
		email.Html = html
	}
}

// WithVariable option assigns a value to the given template variable in the email template.
func WithVariable(key string, value any) EmailOption {
	return func(email *Email) {
		email.mu.Lock()
		defer email.mu.Unlock()
		email.template.Variables[key] = value
	}
}

// WithTag option applies the given tag to the email.
func WithTag(key string, value string) EmailOption {
	return func(email *Email) {
		email.mu.Lock()
		defer email.mu.Unlock()
		email.tags = append(email.tags, resend.Tag{Name: key, Value: value})
	}
}

func WithExistingEmail(data *Email) EmailOption {
	return func(email *Email) {
		if data.Email != nil {
			email.Email = data.Email
		}
		if data.template != nil {
			email.template = data.template
		}
		if len(data.tags) > 0 {
			email.tags = data.tags
		}
	}
}

// SendEmail sends the given email.
func SendEmail(ctx context.Context, options ...EmailOption) error {
	client, err := loadClient()
	if err != nil {
		return fmt.Errorf("load client: %w", err)
	}

	email := &Email{
		Email:    &resend.Email{},
		template: &resend.EmailTemplate{},
	}
	for option := range slices.Values(options) {
		option(email)
	}

	if err := email.Valid(); err != nil {
		return fmt.Errorf("email is not valid: %w", err)
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
		if err != nil && !errors.Is(err, &resend.RateLimitError{}) {
			return resp, fmt.Errorf("batch failed: %w", err)
		}
		if rateLimitErr, ok := errors.AsType[*resend.RateLimitError](
			err,
		); ok &&
			rateLimitErr.Message == "rate_limit_exceeded" {
			if seconds, err := strconv.Atoi(rateLimitErr.RetryAfter); err != nil {
				return nil, backoff.RetryAfter(seconds)
			}
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
