// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:get-return
package server

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/server/handlers"
)

// Ensures we statisfy the ServerInterface interface.
var _ ServerInterface = (*Server)(nil)

// GetLogin handles login for provider.
// (GET /login/{provider}).
func (s Server) GetLogin(res http.ResponseWriter, req *http.Request, provider string) {
	logger := s.Logger.With(slog.String("handler", chi.RouteContext(req.Context()).RoutePath))
	ctx := logging.ToContext(req.Context(), logger)

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
	logger := s.Logger.With(slog.String("handler", chi.RouteContext(req.Context()).RoutePath))
	ctx := logging.ToContext(req.Context(), logger)

	if params.Code == "" {
		logger.Error("Invalid code.")
		res.WriteHeader(http.StatusUnauthorized)
		return
	}

	if params.State == "" {
		logger.Error("Invalid state.")
		res.WriteHeader(http.StatusUnauthorized)
		return
	}

	switch provider {
	case "auth0":
		handlers.Auth0Callback(res, req.WithContext(ctx), s.API.auth, params.Code, params.State)
	default:
		logger.Warn("No provider to satisfy callback.")
		http.NotFound(res, req)
	}
	// Redirect to logged in page.
	req.Header.Add("Content-Type", "")
	http.Redirect(res, req.WithContext(ctx), "/home", http.StatusTemporaryRedirect)
}

// GetLogout handles logging user out from specified provider.
// (GET /logout/{provider}).
func (s Server) GetLogout(res http.ResponseWriter, req *http.Request, provider string) {
	logger := s.Logger.With(slog.String("handler", chi.RouteContext(req.Context()).RoutePath))

	switch provider {
	case "auth0":
		handlers.Auth0LogoutHandler(res, req, s.API.auth)
	default:
		logger.Error("No provider to satisfy login.")
		http.NotFound(res, req)
	}
}

// GetIndex serves the front page.
// GET(/).
func (s Server) GetIndex(res http.ResponseWriter, req *http.Request) {
	handlers.Index(res, req)
}
