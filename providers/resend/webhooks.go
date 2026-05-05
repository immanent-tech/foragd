// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package resend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/mail"
	"slices"

	"github.com/resend/resend-go/v3"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/service"
)

// HandleWebhook will handle incoming webhook requests from Resend.
func HandleWebhook(res http.ResponseWriter, req *http.Request) {
	const maxBodyBytes = int64(65536)
	bodyReader := http.MaxBytesReader(res, req.Body, maxBodyBytes)
	body, err := io.ReadAll(bodyReader)
	if err != nil {
		slogctx.FromCtx(req.Context()).Error("Error reading webhook request body.",
			slog.Any("error", err),
		)
		res.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	client, err := loadClient()
	if err != nil {
		slogctx.FromCtx(req.Context()).Error("Error loading resend client.",
			slog.Any("error", err),
		)
		res.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	// Extract Svix headers
	headers := resend.WebhookHeaders{
		Id:        req.Header.Get("svix-id"),
		Timestamp: req.Header.Get("svix-timestamp"),
		Signature: req.Header.Get("svix-signature"),
	}

	// Verify the webhook
	err = client.Webhooks.Verify(&resend.VerifyWebhookOptions{
		Payload:       string(body),
		Headers:       headers,
		WebhookSecret: cfg.WebHookSecret,
	})

	if err != nil {
		slogctx.FromCtx(req.Context()).Error("Webhook verification failed.",
			slog.Any("error", err),
		)
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	// Parse the verified payload
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		slogctx.FromCtx(req.Context()).Error("Unable to parse received webhook body.",
			slog.Any("error", err),
		)
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	switch payload["type"] {
	case "email.received":
		slogctx.FromCtx(req.Context()).Debug("Email received",
			slog.String("type", payload["type"].(string)),
			slog.Any("payload", payload),
		)
		var email WebhookEmailReceieved
		if err := json.Unmarshal(body, &email); err != nil {
			slogctx.FromCtx(req.Context()).Error("Unable to parse email.recieved webhook body.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := handleRecievedEmail(req.Context(), client, email.Data); err != nil {
			slogctx.FromCtx(req.Context()).Error("Error occured processing received email.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
	default:
		slogctx.FromCtx(req.Context()).Warn("Recieved unhandled webhook",
			slog.String("type", payload["type"].(string)),
			slog.Any("payload", payload),
		)
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	json.NewEncoder(res).Encode(map[string]bool{"success": true})
}

// handleRecievedEmail processes an incoming email. If it is addressed to a user address, the email is extracted and
// indexed as a new email subscritpion article. Otherwise, if it is addressed to our catch-all/admin address, it is
// forwarded. All other emails are ignored.
func handleRecievedEmail(ctx context.Context, client *resend.Client, details EmailRecieved) error {
	// Match the email to address to a user subscription email
	user, err := service.GetUserBySubscriptionEmail(ctx, details.To...)
	if err != nil {
		// If this does not match a user email, process as a non-user email
		if apiErr, ok := errors.AsType[*models.APIError](err); ok && apiErr.StatusCode == http.StatusNotFound {
			return handleNonUserEmail(ctx, client, details)
		}
		return fmt.Errorf("get user by subscription email: %w", err)
	}
	// Load user data into context for later methods.
	ctx = models.UserToCtx(ctx, user)

	from, err := mail.ParseAddress(details.From)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to parse from address. Deriving manually.",
			slog.Any("error", err),
		)
		from = &mail.Address{
			Address: details.From,
		}
	}

	// Retrieve (and/or create) an EmailSubscription for this user and sender.
	subscription, err := service.GetEmailSubscription(ctx, user, from)
	if err != nil {
		return fmt.Errorf("get email subscription: %w", err)
	}

	// Retrieve the full email content and details.
	rawEmail, err := client.Emails.Receiving.GetWithContext(ctx, details.EmailId)
	if err != nil {
		return fmt.Errorf("get email details: %w", err)
	}

	// Parse into our custom format and ensure its valid.
	email := &ReceivedEmail{ReceivedEmail: rawEmail}
	if err := email.Valid(); err != nil {
		return fmt.Errorf("parse email contents: %w", err)
	}

	// Create an Item from the email and index it.
	item := models.NewEmailItem(email, subscription)
	if err := models.AddItems(ctx, *item); err != nil {
		return fmt.Errorf("add email item: %w", err)
	}
	return nil
}

// handleNonUserEmail handles emails not addressed to user addresses. If they are addressed to our catch-all/admin
// address, they are forwarded to that address. Otherwise, they are ignored.
func handleNonUserEmail(ctx context.Context, client *resend.Client, details EmailRecieved) error {
	if err := loadConfig(); err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Ignore emails not explicitly addressed to our admin/catch-all address.
	if !slices.Contains(details.To, cfg.ReplyToEmail) {
		return nil
	}

	// Retrieve the full email content and details.
	rawEmail, err := client.Emails.Receiving.GetWithContext(ctx, details.EmailId)
	if err != nil {
		return fmt.Errorf("get email details: %w", err)
	}

	// Parse into our custom format and ensure its valid.
	email := &ReceivedEmail{ReceivedEmail: rawEmail}
	if err := email.Valid(); err != nil {
		return fmt.Errorf("parse email contents: %w", err)
	}

	// Forward the email.
	if err := email.Forward(ctx, cfg.AdminEmail); err != nil {
		return fmt.Errorf("forward non-user email: %w", err)
	}

	return nil
}
