// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package resend

import (
	"context"
	"fmt"
	"log/slog"
	"net/mail"
	"slices"
	"time"

	"github.com/resend/resend-go/v3"
	slogctx "github.com/veqryn/slog-context"

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
	var (
		from *mail.Address
		err  error
	)
	if fromHdr, ok := e.Headers["from"]; ok {
		from, err = mail.ParseAddress(fromHdr)
	} else {
		from, err = mail.ParseAddress(e.From)
	}
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
		return fmt.Errorf("%w: empty subject", ErrInvalidEmail)
	}
	if e.GetBody() == "" {
		return fmt.Errorf("%w: empty body", ErrInvalidEmail)
	}
	return nil
}

// ExtractAttachments will extract and return the attachments on the email, if any.
func (e *ReceivedEmail) ExtractAttachments(ctx context.Context) ([]*resend.Attachment, error) {
	attachments := make([]*resend.Attachment, 0, len(e.Attachments))
	client, err := loadClient()
	if err != nil {
		return nil, fmt.Errorf("load client: %w", err)
	}
	for attachment := range slices.Values(e.Attachments) {
		a, err := client.Emails.GetAttachmentWithContext(ctx, e.Id, attachment.Id)
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Could not fetch attachment",
				slog.String("email_id", e.Id),
				slog.String("attachment_id", attachment.Id),
				slog.Any("error", err),
			)
		} else {
			attachments = append(attachments,
				&resend.Attachment{
					Path:        a.DownloadUrl,
					Filename:    a.Filename,
					ContentType: a.ContentType,
					ContentId:   a.ContentId,
				})
		}
	}
	return attachments, nil
}

// Forward will forward the recieved email to the given addresses.
func (e *ReceivedEmail) Forward(ctx context.Context, to ...string) error {
	client, err := loadClient()
	if err != nil {
		return fmt.Errorf("load client: %w", err)
	}
	attachments, err := e.ExtractAttachments(ctx)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Could not extract some attachments.",
			slog.Any("error", err),
		)
	}
	params := &resend.SendEmailRequest{
		To:          to,
		From:        "no-reply@foragd.app",
		ReplyTo:     e.From,
		Subject:     e.Subject,
		Html:        e.Html,
		Text:        e.Text,
		Attachments: attachments,
	}

	resp, err := client.Emails.SendWithContext(ctx, params)
	if err != nil {
		return fmt.Errorf("forward email: %w", err)
	}

	slogctx.FromCtx(ctx).Debug("Forwarded email.",
		slog.String("id", resp.Id),
	)
	return nil
}
