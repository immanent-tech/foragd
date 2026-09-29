/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package resend

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/mail"
	"slices"
	"time"

	"github.com/immanent-tech/go-base/validation"
	"github.com/resend/resend-go/v3"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/service"
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
func (e *ReceivedEmail) Validate() error {
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

		if a, err := client.Emails.GetAttachmentWithContext(ctx, e.Id, attachment.Id); err != nil {
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

type WebhookEmailReceieved struct {
	Type      string        `json:"type,omitempty"`
	CreatedAt string        `json:"created_at,omitempty"`
	Data      EmailRecieved `json:"data,omitempty"`
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

type UserService interface {
	GetUserBySubscriptionEmail(ctx context.Context, emails ...string) (*models.User, error)
}

type SubscriptionsService interface {
	GetAllSubscriptions(ctx context.Context) (models.Subscriptions, error)
	AddSubscriptions(ctx context.Context, subscriptions ...*models.Subscription) error
}

type ItemService interface {
	AddItems(ctx context.Context, items models.Items) (map[string]models.Items, error)
}

// SubscriptionFactory wraps service.NewEmailSubscription.
type SubscriptionFactory func(ctx context.Context, userID string, from *mail.Address) (*models.Subscription, error)

type Processor struct {
	subs   SubscriptionsService
	users  UserService
	items  ItemService
	newSub SubscriptionFactory
}

func NewReceivedEmailProcessor(
	subs SubscriptionsService, users UserService, items ItemService,
	newSub SubscriptionFactory,
) *Processor {
	return &Processor{subs, users, items, newSub}
}

func (p *Processor) ProcessReceived(ctx context.Context, details EmailRecieved) error {
	user, err := p.users.GetUserBySubscriptionEmail(ctx, details.To...)
	if err != nil {
		if apiErr, ok := errors.AsType[*models.APIError](err); ok && apiErr.StatusCode == http.StatusNotFound {
			return p.handleNonUser(ctx, &details)
		}
		return fmt.Errorf("get user by subscription email: %w", err)
	}

	if user.Metadata.NewsletterLimit != nil && user.Metadata.NewsletterLimit.Exceeded {
		return models.ErrEmailNewsletterLimitExceeded
	}

	from, err := mail.ParseAddress(details.From)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to parse from address. Deriving manually.",
			slog.Any("error", err),
		)
		from = &mail.Address{
			Address: details.From,
		}
	}

	// Try to find an existing subscription for this email newsletter.
	var subscription *models.Subscription
	allSubscriptions, err := p.subs.GetAllSubscriptions(ctx)
	if err != nil {
		return fmt.Errorf("get user subscriptions: %w", err)
	}
	emailSubscriptions := allSubscriptions.FilterEmailIDs(from.Address)
	if len(emailSubscriptions) == 0 {
		// Create a new email subscription for this newsletter.
		var err error
		subscription, err = service.NewEmailSubscription(ctx, user.GetID(), from)
		if err != nil {
			return fmt.Errorf("create email subscription: %w", err)
		}
		// Add the new subscription.
		if err := p.subs.AddSubscriptions(ctx, subscription); err != nil {
			return fmt.Errorf("add email subscription: %w", err)
		}
	} else {
		subscription = emailSubscriptions[0]
	}

	// Retrieve the full email content and details.
	email, err := GetFullEmail(ctx, details.EmailId)
	if err != nil {
		return fmt.Errorf("parse email: %w", err)
	}
	if err := email.Validate(); err != nil {
		return fmt.Errorf("validate email: %w", err)
	}

	// Create an Item from the email and index it.
	item := models.NewEmailItem(email, subscription)
	if _, err := p.items.AddItems(ctx, models.Items{item}); err != nil {
		return fmt.Errorf("add email item: %w", err)
	}
	return nil
}

func (p *Processor) handleNonUser(ctx context.Context, details *EmailRecieved) error {
	valid, err := IsValidReplyTo(details.To)
	if !valid {
		return fmt.Errorf("check valid reply to: %w", err)
	}
	if err := ForwardAdminEmail(ctx, details); err != nil {
		return fmt.Errorf("forward admin email: %w", err)
	}
	return nil
}
