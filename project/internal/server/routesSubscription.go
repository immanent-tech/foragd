// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:get-return
package server

import (
	"log/slog"
	"net/http"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/server/handlers"
)

// SubscriptionAdd handles subscription request input GET(/subscription/add).
func (s Server) GetSubscriptionAdd(res http.ResponseWriter, req *http.Request) {
	ctx := logging.ToContext(req.Context(), s.Logger.With(slog.String("handler", "SubscriptionAdd")))

	handlers.AddSubscriptionHandler(res, req.WithContext(ctx))
}

// SubscriptionAddSubmit processes a subscription request POST(/subscription/add)
func (s Server) PostSubscriptionAdd(res http.ResponseWriter, req *http.Request) {
	ctx := logging.ToContext(req.Context(), s.Logger.With(slog.String("handler", "SubscriptionAddSubmit")))

	handlers.ProcessAddSubscriptionForm(res, req.WithContext(ctx), s.API.elastic, s.API.pg)
}

// SubscriptionValidate validates a subscription request GET(/subscription/validate)
func (s Server) PostSubscriptionValidate(res http.ResponseWriter, req *http.Request) {
	ctx := logging.ToContext(req.Context(), s.Logger.With(slog.String("handler", "SubscriptionValidate")))

	handlers.Validate(res, req.WithContext(ctx), handlers.UpdateAddSubscriptionForm)
}

// Edit a new subscription
// (GET /home/subscription/edit/{subID})
func (s Server) GetSubscriptionEdit(w http.ResponseWriter, r *http.Request, subID string) {
	w.WriteHeader(http.StatusNotImplemented)
}

// Process a subscription edit
// (POST /home/subscription/edit/{subID})
func (s Server) PostSubscriptionEdit(w http.ResponseWriter, r *http.Request, subID string) {
	w.WriteHeader(http.StatusNotImplemented)
}
