// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package resend

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"

	"github.com/resend/resend-go/v3"
	slogctx "github.com/veqryn/slog-context"
)

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
