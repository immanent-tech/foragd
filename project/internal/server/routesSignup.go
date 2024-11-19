// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:get-return
package server

import (
	"log/slog"
	"net/http"

	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/server/handlers"
)

// SignUp serves the user sign up page.
// GET(/signup).
func (s Server) GetSignup(res http.ResponseWriter, req *http.Request) {
	// if !htmx.IsHTMX(req) {
	// 	s.Logger.Error("Request was not made by htmx.", slog.String("handler", "ValidateSignup"))
	// 	http.Error(res, "Invalid request", http.StatusBadRequest)
	// 	return
	// }

	ctx := logging.ToContext(req.Context(), s.Logger.With(slog.String("handler", "Signup")))

	handlers.Signup(res, req.WithContext(ctx))
}

// PostSignup processes the user sign up request.
// POST(/signup).
func (s Server) PostSignup(res http.ResponseWriter, req *http.Request) {
	if !htmx.IsHTMX(req) {
		s.Logger.Error("Request was not made by htmx.", slog.String("handler", "ValidateSignup"))
		http.Error(res, "Invalid request", http.StatusBadRequest)
		return
	}

	ctx := logging.ToContext(req.Context(), s.Logger.With(slog.String("handler", "Signup")))

	handlers.ProcessSignup(res, req.WithContext(ctx), s.API.user, s.API.pg)
}

func (s Server) PostSignupValidate(res http.ResponseWriter, req *http.Request) {
	if !htmx.IsHTMX(req) {
		s.Logger.Error("Request was not made by htmx.", slog.String("handler", "ValidateSignup"))
		http.Error(res, "Invalid request", http.StatusBadRequest)
		return
	}

	ctx := logging.ToContext(req.Context(), s.Logger.With(slog.String("handler", "Signup")))

	handlers.Validate(res, req.WithContext(ctx), handlers.UpdateSignupInput)
}
