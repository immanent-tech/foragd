// Copyright (C) 2024 Joshua Rich <joshua.rich@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package server

import (
	"log/slog"
	"net/http"

	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/server/handlers"
)

// Ensures we statisfy the ServerInterface interface.
var _ ServerInterface = (*Server)(nil)

// UserLogin handles login for provider.
// (GET /login/{provider})
func (s Server) UserLogin(res http.ResponseWriter, req *http.Request, provider string) {
	ctx := logging.ToContext(req.Context(), s.Logger.With(slog.String("handler", "UserLogin")))

	switch provider {
	case "auth0":
		handlers.Auth0Login(res, req.WithContext(ctx), s.API.auth)
	default:
		s.Logger.Warn("No provider to satisfy login.")
		http.NotFound(res, req)
	}
}

// UserLoginCallback handles callback from provider.
// (GET /login/{provider}/callback)
func (s Server) UserLoginCallback(res http.ResponseWriter, req *http.Request, provider string, params UserLoginCallbackParams) {
	ctx := logging.ToContext(req.Context(), s.Logger.With(slog.String("handler", "UserLoginCallback")))

	if params.Code == "" {
		logging.FromContext(req.Context()).
			Error("Invalid code.")
		res.WriteHeader(http.StatusUnauthorized)
		return
	}

	if params.State == "" {
		logging.FromContext(req.Context()).
			Error("Invalid state.")
		res.WriteHeader(http.StatusUnauthorized)
		return
	}

	switch provider {
	case "auth0":
		handlers.Auth0Callback(res, req.WithContext(ctx), s.API.auth, params.Code, params.State)
	default:
		s.Logger.Warn("No provider to satisfy callback.")
		http.NotFound(res, req)
	}
}

// UserLogout handles logging user out from specified provider.
// (GET /logout/{provider})
func (s Server) UserLogout(res http.ResponseWriter, req *http.Request, provider string) {
	ctx := logging.ToContext(req.Context(), s.Logger.With(slog.String("handler", "UserLogout")))

	switch provider {
	case "auth0":
		handlers.Auth0LogoutHandler(res, req.WithContext(ctx), s.API.auth)
	default:
		logging.LogReq(req, http.StatusNotFound).
			Error("No provider to statisfy login.")
		http.NotFound(res, req)
	}
}

// Index serves the front page.
// GET(/)
func (s Server) Index(res http.ResponseWriter, req *http.Request) {
	handlers.Index(res, req)
}

// SignUp serves the user sign up page.
// GET(/signup)
func (s Server) Signup(res http.ResponseWriter, req *http.Request) {
	if !htmx.IsHTMX(req) {
		s.Logger.Error("Request was not made by htmx.", slog.String("handler", "ValidateSignup"))
		http.Error(res, "Invalid request", http.StatusBadRequest)
		return
	}

	ctx := logging.ToContext(req.Context(), s.Logger.With(slog.String("handler", "Signup")))

	handlers.Signup(res, req.WithContext(ctx))
}

// ProcessSignUp processes the user sign up request.
// POST(/signup)
func (s Server) ProcessSignup(res http.ResponseWriter, req *http.Request) {
	if !htmx.IsHTMX(req) {
		s.Logger.Error("Request was not made by htmx.", slog.String("handler", "ValidateSignup"))
		http.Error(res, "Invalid request", http.StatusBadRequest)
		return
	}

	ctx := logging.ToContext(req.Context(), s.Logger.With(slog.String("handler", "Signup")))

	handlers.ProcessSignup(res, req.WithContext(ctx), s.API.user, s.API.pg)
}

func (s Server) ValidateSignup(res http.ResponseWriter, req *http.Request) {
	if !htmx.IsHTMX(req) {
		s.Logger.Error("Request was not made by htmx.", slog.String("handler", "ValidateSignup"))
		http.Error(res, "Invalid request", http.StatusBadRequest)
		return
	}

	ctx := logging.ToContext(req.Context(), s.Logger.With(slog.String("handler", "Signup")))

	handlers.Validate(res, req.WithContext(ctx), handlers.UpdateSignupInput)
}

// AddItem serves the add item modal.
// GET(/signup)
func (s Server) AddItem(res http.ResponseWriter, req *http.Request) {
	if !htmx.IsHTMX(req) {
		s.Logger.Error("Request was not made by htmx.", slog.String("handler", "ValidateSignup"))
		http.Error(res, "Invalid request", http.StatusBadRequest)
		return
	}

	ctx := logging.ToContext(req.Context(), s.Logger.With(slog.String("handler", "Signup")))

	handlers.AddItem(res, req.WithContext(ctx))
}

// ProcessAddItem processes the add item request.
// POST(/signup)
func (s Server) ProcessAddItem(res http.ResponseWriter, req *http.Request) {
	if !htmx.IsHTMX(req) {
		s.Logger.Error("Request was not made by htmx.", slog.String("handler", "ValidateSignup"))
		http.Error(res, "Invalid request", http.StatusBadRequest)
		return
	}

	ctx := logging.ToContext(req.Context(), s.Logger.With(slog.String("handler", "Signup")))

	handlers.ProcessAddItem(res, req.WithContext(ctx), s.API.pg)
}

func (s Server) ValidateAddItem(res http.ResponseWriter, req *http.Request) {
	if !htmx.IsHTMX(req) {
		s.Logger.Error("Request was not made by htmx.", slog.String("handler", "ValidateSignup"))
		http.Error(res, "Invalid request", http.StatusBadRequest)
		return
	}

	ctx := logging.ToContext(req.Context(), s.Logger.With(slog.String("handler", "Signup")))

	handlers.Validate(res, req.WithContext(ctx), handlers.UpdateAddItemForm)
}
