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

// Ensures we statisfy the ServerInterface interface.
var _ ServerInterface = (*Server)(nil)

// GetLogin handles login for provider.
// (GET /login/{provider}).
func (s Server) GetLogin(res http.ResponseWriter, req *http.Request, provider string) {
	ctx := logging.ToContext(req.Context(), s.Logger.With(slog.String("handler", "UserLogin")))

	switch provider {
	case "auth0":
		handlers.Auth0Login(res, req.WithContext(ctx), s.API.auth)
	default:
		s.Logger.Warn("No provider to satisfy login.")
		http.NotFound(res, req)
	}
}

// GetLoginCallback handles callback from provider.
// (GET /login/{provider}/callback).
func (s Server) GetLoginCallback(res http.ResponseWriter, req *http.Request, provider string, params GetLoginCallbackParams) {
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

// GetLogout handles logging user out from specified provider.
// (GET /logout/{provider}).
func (s Server) GetLogout(res http.ResponseWriter, req *http.Request, provider string) {
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

// GetIndex serves the front page.
// GET(/).
func (s Server) GetIndex(res http.ResponseWriter, req *http.Request) {
	handlers.Index(res, req)
}
