// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:get-return
package server

import (
	"log/slog"
	"net/http"

	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/server/handlers"
	"github.com/joshuar/go-feed-me/web/templates/partials"
)

// SignUp serves the user sign up page.
// GET(/signup).
func (s Server) GetSignup(res http.ResponseWriter, req *http.Request) {
	logger := s.Logger.With(slog.String("handler", "Signup"))

	if err := htmx.NewResponse().RenderTempl(req.Context(), res, partials.ShowSignUpForm()); err != nil {
		logger.Error("Cannot render signup form.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// PostSignup processes the user sign up request.
// POST(/signup).
func (s Server) PostSignup(res http.ResponseWriter, req *http.Request) {
	// ctx := logging.ToContext(req.Context(), s.Logger.With(slog.String("handler", "Signup")))

	newUser, problems, err := handlers.DecodeForm[*models.APIUser](req)
	if err != nil && len(problems) == 0 {
		logging.FromContext(req.Context()).
			Error("Could not decode submitted signup request.", slog.Any("error", err))
		handlers.Validate(res, req, partials.UpdateSignupInput)
		return
	}

	// Create the user in the auth backend.
	userID, err := s.API.user.Create(req.Context(), newUser)
	if err != nil {
		logging.FromContext(req.Context()).
			Error("Could not create user account.", slog.Any("error", err))

		if err = htmx.NewResponse().RenderTempl(req.Context(), res, partials.SignupError()); err != nil {
			logging.FromContext(req.Context()).
				Error("Cannot render sign up error.", slog.Any("error", err))
			res.WriteHeader(http.StatusInternalServerError)
		}

		return
	}

	// Create new user in the database backend.
	err = s.API.pg.AddUser(req.Context(), userID, newUser)
	if err != nil {
		logging.FromContext(req.Context()).
			Error("Could not create user account.", slog.Any("error", err))

		if err = htmx.NewResponse().RenderTempl(req.Context(), res, partials.SignupError()); err != nil {
			logging.FromContext(req.Context()).
				Error("Cannot render sign up error.", slog.Any("error", err))
			res.WriteHeader(http.StatusInternalServerError)
		}
	}

	// Show success message.
	if err := htmx.NewResponse().RenderTempl(req.Context(), res, partials.SignupSuccess()); err != nil {
		logging.FromContext(req.Context()).
			Error("Cannot render sign up error.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)
	}
}

func (s Server) PostSignupValidate(res http.ResponseWriter, req *http.Request) {
	ctx := logging.ToContext(req.Context(), s.Logger.With(slog.String("handler", "Signup")))

	handlers.Validate(res, req.WithContext(ctx), partials.UpdateSignupInput)
}
