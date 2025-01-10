// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:get-return
package server

import (
	"net/http"
)

// SubscriptionAdd handles subscription request input GET(/subscription/add).
func (s Server) GetSubscriptionAdd(res http.ResponseWriter, req *http.Request) {
	// logger := s.Logger.With(slog.String("handler", "SubscriptionAdd"))

	// if err := modals.CommandModal(req, res, partials.AddSubscriptionForm().Show()); err != nil {
	// 	logger.Warn("Unable to command modal.", slog.Any("error", err))
	// 	res.WriteHeader(http.StatusInternalServerError)
	// }
}

// SubscriptionAddSubmit processes a subscription request POST(/subscription/add)
func (s Server) PostSubscriptionAdd(res http.ResponseWriter, req *http.Request) {
	// logger := s.Logger.With(slog.String("handler", "SubscriptionAddSubmit"))

	// newSubscription, problems, err := forms.DecodeForm[*models.APISubscription](req)
	// if err != nil && len(problems) == 0 {
	// 	logging.FromContext(req.Context()).
	// 		Error("Could not decode submitted add feed request.", slog.Any("error", err))
	// 	forms.Validate(res, req, partials.UpdateAddSubscriptionForm)
	// 	res.WriteHeader(http.StatusInternalServerError)
	// 	return
	// }

	// userID, err := session.UserID(req.Context())
	// if err != nil {
	// 	logger.Error("Could not retrieve user ID from session.")
	// 	res.WriteHeader(http.StatusInternalServerError)
	// 	return
	// }

	// if err := models.AddNewSubscription(req.Context(), userID, s.API.elastic, s.API.pg, newSubscription); err != nil {
	// 	logger.Error("Could not add item.", slog.Any("error", err))
	// 	res.WriteHeader(http.StatusInternalServerError)
	// 	return
	// }

	// res.WriteHeader(http.StatusOK)
}

// SubscriptionValidate validates a subscription request GET(/subscription/validate)
func (s Server) PostSubscriptionValidate(res http.ResponseWriter, req *http.Request) {
	// ctx := logging.ToContext(req.Context(), s.Logger.With(slog.String("handler", "SubscriptionValidate")))

	// forms.Validate(res, req.WithContext(ctx), partials.UpdateAddSubscriptionForm)
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
