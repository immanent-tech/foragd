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
	"sync"

	"github.com/resend/resend-go/v3"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
)

var ErrInvalidEmail = errors.New("email is invalid")

var loadClient = sync.OnceValues(func() (*resend.Client, error) {
	if err := loadConfig(); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	client := resend.NewClient(cfg.APIKey)
	return client, nil
})

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
		slogctx.FromCtx(req.Context()).Warn("Email received",
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

func handleRecievedEmail(ctx context.Context, client *resend.Client, details EmailRecieved) error {
	// Match the email to address to a user subscription email
	user, err := models.GetUserBySubscriptionEmail(ctx, details.To...)
	if err != nil {
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
	subscription, err := models.GetEmailSubscription(ctx, user.GetID(), from)
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
	if err := models.AddItems(ctx, item); err != nil {
		return fmt.Errorf("add email item: %w", err)
	}
	return nil
}
