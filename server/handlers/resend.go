/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package handlers

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/validation"

	"github.com/immanent-tech/foragd/providers/resend"
)

// WebhookVerifier checks the Svix signature on an incoming request.
type WebhookVerifier interface {
	Verify(req *http.Request, body []byte) error
}

// EmailReceiver handles a verified email.received event.
type EmailReceiver interface {
	Process(ctx context.Context, details resend.EmailRecieved) error
}

// HandleResendWebhook will handle incoming webhook requests from Resend.
func HandleResendWebhook(verifier WebhookVerifier, processor EmailReceiver) http.HandlerFunc {
	const maxBodyBytes = int64(65536)
	return func(res http.ResponseWriter, req *http.Request) {
		log := slogctx.FromCtx(req.Context())

		body, err := io.ReadAll(http.MaxBytesReader(res, req.Body, maxBodyBytes))
		if err != nil {
			log.Error("Error reading webhook request body.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		// Verify webhook data.
		if err := verifier.Verify(req, body); err != nil {
			log.Error("Webhook verification failed.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		// Parse the verified payload.
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			log.Error("Unable to parse received webhook body.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		// Extract the "envelope".
		var envelope struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			log.Error("Unable to parse received webhook body.", slog.Any("error", err))
			res.WriteHeader(http.StatusBadRequest)
			return
		}
		envelope.Type = validation.SanitizeString(envelope.Type)
		envelope.Data = validation.SanitizeBytes(envelope.Data)

		// Act accordingly based on type.
		switch envelope.Type {
		case "email.received":
			var details resend.EmailRecieved
			if err := json.Unmarshal(envelope.Data, &details); err != nil {
				log.Error("Unable to parse email.received webhook body.", slog.Any("error", err))
				res.WriteHeader(http.StatusBadRequest)
				return
			}
			if err := processor.Process(req.Context(), details); err != nil {
				log.Error("Error occurred processing received email.", slog.Any("error", err))
				res.WriteHeader(http.StatusOK)
				return
			}
		default:
			log.Warn("Received unhandled webhook", slog.String("type", envelope.Type))
		}

		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(res).Encode(map[string]bool{"success": true})
	}
}
